package token

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	defaultExpiry = 15 * time.Minute
	defaultUses   = 1
)

type TokenType string

const (
	TokenClaim TokenType = "claim"
	TokenCert  TokenType = "cert"
)

// CreateTester runs `skupper token create` command, validating
// the output as well as asserting token file has been created.
type CreateTester struct {
	Name            string
	FileName        string
	Expiry          string
	Password        string
	Type            TokenType
	Uses            string
	LocalOnly       bool
	PostDelay       time.Duration
	PolicyProhibits bool
}

func (t *CreateTester) Command(cluster *base.ClusterContext) []string {
	args := cli.SkupperCommonOptions(cluster)
	args = append(args, "token", "create", t.FileName)

	if t.Name != "" {
		args = append(args, "--name", t.Name)
	}
	if t.Expiry != "" {
		args = append(args, "--expiry", t.Expiry)
	}
	if t.Password != "" {
		args = append(args, "--password", t.Password)
	}
	if t.Type != "" {
		args = append(args, "--token-type", string(t.Type))
	}
	if t.Uses != "" {
		args = append(args, "--uses", t.Uses)
	}

	return args
}

func (t *CreateTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	// Execute token create command
	stdout, stderr, err = cli.RunSkupperCli(t.Command(cluster))
	if err != nil {
		log.Println("Validating token creation failing by policy")
		if t.PolicyProhibits && strings.Contains(stderr, "Policy validation error: incoming links are not allowed") {
			err = nil
			return
		}
		return
	} else {
		if t.PolicyProhibits {
			err = fmt.Errorf("Token creation was expected to fail, but it didn't")
			return
		}
	}

	// Validating output
	log.Printf("Validating 'skupper token create'")

	log.Println("validating stdout")
	expectedOutput := fmt.Sprintf("Token written to %s", t.FileName)
	if t.Type == TokenCert {
		expectedOutput = fmt.Sprintf("Connection token written to %s", t.FileName)
	}
	if t.LocalOnly {
		expectedOutput += " (Note: token will only be valid for local cluster)"
	}
	if !strings.Contains(stdout, expectedOutput) {
		err = fmt.Errorf("output did not match - expected: %s - found: %s", expectedOutput, stdout)
		return
	}

	// Validating that token file exists
	log.Println("validating token file")
	_, err = os.Stat(t.FileName)
	if err != nil {
		err = fmt.Errorf("token file was not created - %v", err)
		return
	}

	// Loading secret
	var secret v1.Secret
	var secretFile []byte
	secretFile, err = ioutil.ReadFile(t.FileName)
	if err != nil {
		err = fmt.Errorf("unable to read token file - %v", err)
		return
	}
	yamlS := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	yamlS.Decode(secretFile, nil, &secret)

	// Validating name
	log.Println("validating token name")
	if t.Name != "" && secret.ObjectMeta.Name != t.Name {
		err = fmt.Errorf("invalid token name [expected: %s - found: %s]", t.Name, secret.Name)
		return
	} else if t.Name == "" {
		var matched bool
		matched, err = regexp.MatchString("conn[0-9]+", secret.Name)
		if err != nil {
			return
		}
		if !matched {
			err = fmt.Errorf("unexpected token name: %s", secret.Name)
		}
	}

	// Validating type
	tokenQualifier := secret.Labels[types.SkupperTypeQualifier]
	if t.Type == TokenCert {
		if tokenQualifier != "connection-token" {
			err = fmt.Errorf("incorrect token qualifier - [expected: connection-token - found: %s]", tokenQualifier)
			return
		}

		// At this point we can return as there is nothing else to be validated
		// on a certificate token type
		return
	}

	// From this point on all validations are considering token type as claim
	if tokenQualifier != "token-claim" {
		err = fmt.Errorf("incorrect token qualifier - [expected: token-claim - found: %s]", tokenQualifier)
		return
	}

	// Identifying token-claim-record name
	log.Println("validating token-claim")
	claimUrl, ok := secret.Annotations[types.ClaimUrlAnnotationKey]
	if !ok {
		err = fmt.Errorf("token claim is missing url")
		return
	}
	parsedClaimUrl, err := url.Parse(claimUrl)
	if err != nil {
		return
	}
	claimRecordName := parsedClaimUrl.Path[1:]

	// Retrieving token-claim-record secret
	tokenClaimRecord, err := cluster.VanClient.KubeClient.CoreV1().Secrets(cluster.Namespace).Get(claimRecordName, v12.GetOptions{})
	if err != nil {
		return
	}

	//
	// Expiry, Uses and Password must be validated inside the token-claim-record secret
	//

	// Validating expiry
	if err = t.validateExpiry(tokenClaimRecord); err != nil {
		return
	}

	// Validating uses
	if err = t.validateUses(tokenClaimRecord); err != nil {
		return
	}

	// Validating password
	if err = t.validatePassword(tokenClaimRecord); err != nil {
		return
	}

	// As this is the last task in this scenario, perform a post delay, if needed.
	// This is useful to test expired claims
	if t.PostDelay > 0 {
		log.Printf("delaying %v", t.PostDelay)
		time.Sleep(t.PostDelay)
	}

	return
}
func (t *CreateTester) validatePassword(tokenClaimRecord *v1.Secret) (err error) {
	log.Println("validating password")
	passwordData, ok := tokenClaimRecord.Data[types.ClaimPasswordDataKey]
	if !ok {
		err = fmt.Errorf("password has not been defined at token-claim-record")
		return
	}
	password := string(passwordData)
	if password == "" {
		err = fmt.Errorf("empty password found at token-claim-record")
		return
	}
	if t.Password != "" && t.Password != password {
		err = fmt.Errorf("token-claim-record's password does not match [expected: %s - found: %s]", t.Password, password)
	}
	return
}

func (t *CreateTester) validateUses(tokenClaimRecord *v1.Secret) (err error) {
	log.Println("validating uses")
	usesStr, ok := tokenClaimRecord.Annotations[types.ClaimsRemaining]
	expectedUses, _ := strconv.Atoi(t.Uses)
	if t.Uses == "" {
		expectedUses = defaultUses
	}
	uses, _ := strconv.Atoi(usesStr)

	if !ok && expectedUses > 0 {
		err = fmt.Errorf("number of uses has been defined but claims-remaining has not been set")
		return
	} else if ok {
		if expectedUses == 0 {
			err = fmt.Errorf("claims-remaining has been set but not expected (uses = 0)")
			return
		}
		if uses != expectedUses {
			err = fmt.Errorf("claims-remaining is incorrect [expected: %d - found: %d]", expectedUses, uses)
		}
	}
	return
}

func (t *CreateTester) validateExpiry(tokenClaimRecord *v1.Secret) (err error) {

	log.Println("validating expiry")
	expectExpiration := !strings.HasPrefix(t.Expiry, "0")
	expirationStr, ok := tokenClaimRecord.Annotations[types.ClaimExpiration]

	if !ok && expectExpiration {
		err = fmt.Errorf("expiry has been defined but token-claim-record does not have an expiration")
		return
	} else if ok {
		if !expectExpiration {
			err = fmt.Errorf("expiration has been defined but Expiry was not set")
			return
		}
		expiration, timeErr := time.Parse(time.RFC3339, expirationStr)
		if timeErr != nil {
			err = timeErr
			return
		}
		expiry := defaultExpiry
		if t.Expiry != "" {
			expiry, err = time.ParseDuration(t.Expiry)
			if err != nil {
				return
			}
		}
		expectedExpiration := time.Now().Add(expiry)
		if !expiration.Before(expectedExpiration) {
			err = fmt.Errorf("expected expiration to happen before (current time plus expiry): %v - but found: %v", expectedExpiration, expiration)
			return
		}
	}
	return
}
