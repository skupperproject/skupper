package podman

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

type LinkHandlerPodman struct {
	cli              *podman.PodmanRestClient
	routerCfgHandler qdr.RouterConfigHandler
	routerManager    domain.RouterEntityManager
	credHandler      *PodmanCredentialHandler
	site             *SitePodman
	redeemer         *domain.ClaimRedeemer
}

func NewLinkHandlerPodman(site *SitePodman, cli *podman.PodmanRestClient) *LinkHandlerPodman {
	l := &LinkHandlerPodman{
		site: site,
		cli:  cli,
	}
	l.routerCfgHandler = NewRouterConfigHandlerPodman(cli)
	l.routerManager = NewRouterEntityManagerPodman(cli)
	l.credHandler = NewPodmanCredentialHandler(cli)
	l.redeemer = domain.NewClaimRedeemer("LinkHandlerPodman", site.GetId(), site.GetVersion(), l.updateClaim, l.log)
	return l
}

func (l *LinkHandlerPodman) updateClaim(claim *corev1.Secret) error {
	var kind string
	if claim.Labels == nil {
		kind = types.TypeClaimRequest
	} else {
		kind = claim.Labels[types.SkupperTypeQualifier]
	}
	_, err := l.credHandler.SaveSecretAsVolume(claim, kind)
	if err != nil {
		return err
	}

	if kind == types.TypeToken {
		tlsCerts, err := l.credHandler.cli.VolumeInspect(SharedTlsCertificates)
		if err != nil {
			return fmt.Errorf("claim has been saved but certificate profile could not be created")
		}

		baseDir := fmt.Sprintf("%s-profile", claim.Name)
		for fileName, encodedContent := range claim.Data {
			_, err = tlsCerts.CreateFile(path.Join(baseDir, fileName), encodedContent, true)
			if err != nil {
				return fmt.Errorf("error creating token certificates for link %s under volume %s - %w", claim.Name, tlsCerts.Name, err)
			}
		}
	}
	return err
}

func (l *LinkHandlerPodman) log(name string, format string, args ...interface{}) {
	msg := fmt.Sprintf("[%s] - "+format, append([]interface{}{name}, args...)...)
	if strings.Contains(msg, "ailed") {
		fmt.Println(msg)
	}
}

func (l *LinkHandlerPodman) Create(secret *corev1.Secret, name string, cost int) error {
	// adjusting secret name
	if name == "" {
		name = domain.GenerateLinkName(l)
	}
	secret.Name = name

	// validating secret
	v, err := l.cli.VolumeInspect(name)
	if err == nil && v != nil {
		return fmt.Errorf("link %s already exists", name)
	}
	if len(secret.Labels) == 0 {
		return fmt.Errorf("invalid Skupper token")
	}
	var kind string
	var ok bool
	if kind, ok = secret.Labels[types.SkupperTypeQualifier]; !ok {
		return fmt.Errorf("unable to determine token type")
	}

	// Verifying token
	err = domain.VerifyToken(secret)
	if err != nil {
		return err
	}

	// Validating token
	if err = domain.VerifyNotSelfOrDuplicate(*secret, l.site.GetId(), l); err != nil {
		return err
	}
	err = domain.VerifySecretCompatibility(l.site.GetVersion(), *secret)
	if err != nil {
		return err
	}

	// saving secret as a volume
	switch kind {
	case types.TypeToken:
		err = l.updateClaim(secret)
	case types.TypeClaimRequest:
		err = l.redeemer.RedeemClaim(secret)
	default:
		return fmt.Errorf("invalid type token")
	}
	if err != nil {
		return err
	}
	// updating the router config
	cfg, err := l.routerCfgHandler.GetRouterConfig()
	if err != nil {
		return fmt.Errorf("error retrieving transport config - %w", err)
	}
	hostname, port := domain.GetTokenEndpoint(l.site, secret)

	profile := qdr.SslProfile{Name: fmt.Sprintf("%s-profile", name)}
	cfg.AddSslProfileWithPath("/etc/skupper-router-certs", profile)
	profile = cfg.SslProfiles[profile.Name]
	role := qdr.RoleInterRouter
	if l.site.IsEdge() {
		role = qdr.RoleEdge
	}
	connector := qdr.Connector{
		Name:           name,
		Role:           role,
		Host:           hostname,
		Port:           port,
		Cost:           int32(cost),
		VerifyHostname: true,
		SslProfile:     profile.Name,
	}
	cfg.AddConnector(connector)
	if err = l.routerCfgHandler.SaveRouterConfig(cfg); err != nil {
		return fmt.Errorf("error saving transport config - %w", err)
	}

	// updating router entities (live)
	if err = l.routerManager.CreateSslProfile(profile); err != nil {
		return fmt.Errorf("error defining sslProfile %s - %w", profile.Name, err)
	}
	if err = l.routerManager.CreateConnector(connector); err != nil {
		return fmt.Errorf("error defining connector %s - %w", connector.Name, err)
	}
	return err
}

func (l *LinkHandlerPodman) IsValidLink(name string) error {
	v, err := l.cli.VolumeInspect(name)
	if err != nil {
		return fmt.Errorf("no such link %q", name)
	}
	if kind, ok := v.GetLabels()[types.SkupperTypeQualifier]; !ok || !utils.StringSliceContains([]string{types.TypeToken, types.TypeClaimRequest}, kind) {
		return fmt.Errorf("%q is not a valid link", name)
	}
	if !container.IsOwnedBySkupper(v.GetLabels()) {
		return fmt.Errorf("link volume %s is not owned by Skupper", name)
	}
	return nil
}

func (l *LinkHandlerPodman) Delete(name string) error {
	// validating link is valid
	if err := l.IsValidLink(name); err != nil {
		return err
	}
	sharedCertsVol, err := l.cli.VolumeInspect(SharedTlsCertificates)
	if err != nil {
		return fmt.Errorf("unable to read %s volume - %w", SharedTlsCertificates, err)
	}

	// removing link from configuration
	cfg, err := l.routerCfgHandler.GetRouterConfig()
	if err != nil {
		return fmt.Errorf("error retrieving transport config - %w", err)
	}
	profileName := fmt.Sprintf("%s-profile", name)
	cfg.RemoveConnector(name)
	cfg.RemoveSslProfile(profileName)
	err = l.routerCfgHandler.SaveRouterConfig(cfg)
	if err != nil {
		return fmt.Errorf("error saving transport config - %w", err)
	}

	// removing link profile from skupper-router-certs volume
	if err = sharedCertsVol.DeleteFile(profileName, true); err != nil {
		return fmt.Errorf("error removing %s certificates from volume %s - %w", profileName, SharedTlsCertificates, err)
	}

	// removing link volume
	if err = l.cli.VolumeRemove(name); err != nil {
		return fmt.Errorf("error removing volume %s - %w", name, err)
	}

	// removing entities from running router
	_ = l.routerManager.DeleteSslProfile(profileName)
	_ = l.routerManager.DeleteConnector(name)
	return nil
}

func (l *LinkHandlerPodman) list(name string) ([]*corev1.Secret, error) {
	vl, err := l.cli.VolumeList()
	if err != nil {
		return nil, err
	}
	var secrets []*corev1.Secret
	for _, v := range vl {
		if name != "" && v.Name != name {
			continue
		}
		if l.IsValidLink(v.Name) != nil {
			continue
		}
		secret, err := l.credHandler.LoadVolumeAsSecret(v)
		if err != nil {
			return nil, fmt.Errorf("error loading volume as secret: %s - %w", v.Name, err)
		}
		secrets = append(secrets, secret)
		if name != "" {
			break
		}
	}
	return secrets, nil
}

func (l *LinkHandlerPodman) List() ([]*corev1.Secret, error) {
	return l.list("")
}

func (l *LinkHandlerPodman) status(name string) ([]types.LinkStatus, error) {
	var ls []types.LinkStatus
	secrets, err := l.list(name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving secrets - %w", err)
	}
	connections, err := l.routerManager.QueryConnections("", false)
	if err != nil {
		return nil, fmt.Errorf("error retrieving router connections - %w", err)
	}
	for _, secret := range secrets {
		ls = append(ls, qdr.GetLinkStatus(secret, l.site.IsEdge(), connections))
	}
	return ls, nil
}
func (l *LinkHandlerPodman) StatusAll() ([]types.LinkStatus, error) {
	return l.status("")
}

func (l *LinkHandlerPodman) Status(name string) (types.LinkStatus, error) {
	var empty types.LinkStatus
	ls, err := l.status(name)
	if err != nil {
		return empty, err
	}
	if len(ls) == 0 {
		return empty, fmt.Errorf("No such link %q", name)
	}
	return ls[0], nil
}

func (l *LinkHandlerPodman) Detail(link types.LinkStatus) (map[string]string, error) {
	status := "Active"

	if !link.Connected {
		status = "Not active"

		if len(link.Description) > 0 {
			status = fmt.Sprintf("%s (%s)", status, link.Description)
		}
	}

	return map[string]string{
		"Name:":    link.Name,
		"Status:":  status,
		"Site:":    l.site.Name + "-" + l.site.Id,
		"Cost:":    strconv.Itoa(link.Cost),
		"Created:": link.Created,
	}, nil
}

func (l *LinkHandlerPodman) RemoteLinks(ctx context.Context) ([]*types.RemoteLinkInfo, error) {
	routers, err := l.routerManager.QueryAllRouters()
	if err != nil {
		return nil, fmt.Errorf("error retrieving remote links - %w", err)
	}
	var remoteLinks []*types.RemoteLinkInfo
	for _, router := range routers {
		if router.Id == l.site.Id {
			continue
		}
		if utils.StringSliceContains(router.ConnectedTo, l.site.Id) {
			remoteLinks = append(remoteLinks, &types.RemoteLinkInfo{
				SiteName: router.Id,
				SiteId:   router.Site.Id,
			})
		}
	}
	return remoteLinks, nil
}
