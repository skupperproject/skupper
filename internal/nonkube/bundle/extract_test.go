package bundle

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelfExtractingBundle_InstallFile(t *testing.T) {
	b := &SelfExtractingBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	assert.Equal(t, b.InstallFile(), "/tmp/skupper-install-my-site.sh")
}

// TestSelfExtractingBundle_Generate unit test the self extract generate
// only, not producing a valid bundle. But it validates if the self-extracting
// bundle is produced and that it is an executable.
func TestSelfExtractingBundle_Generate(t *testing.T) {
	var cleanupPaths []string
	b := &SelfExtractingBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	// cleanup function
	defer func() {
		var errors []error
		appendError := func(e error) {
			if e == nil {
				return
			}
			errors = append(errors, e)
		}
		for _, cleanupPath := range cleanupPaths {
			appendError(os.RemoveAll(cleanupPath))
		}
		assert.Equal(t, len(errors), 0, "No errors expected during cleanup, but found: %v", errors)
	}()
	var sitePath string
	var err error

	t.Run("generate-fake-crs", func(t *testing.T) {
		sitePath, err = fakeSiteCrs(true)
		assert.Assert(t, err)
		cleanupPaths = append(cleanupPaths, sitePath)
	})

	t.Run("generate-self-extracting-bundle", func(t *testing.T) {
		tb := utils.NewTarball()
		assert.Assert(t, tb.AddFiles(sitePath))
		assert.Assert(t, b.Generate(tb))
		cleanupPaths = append(cleanupPaths, b.InstallFile())
	})

	t.Run("validate-install-file", func(t *testing.T) {
		installFileStat, err := os.Stat(b.InstallFile())
		assert.Assert(t, err)
		assert.Assert(t, installFileStat.Mode().IsRegular())
		assert.Assert(t, installFileStat.Mode().Perm() == os.FileMode(0755))

		helpCommand := exec.Command(b.InstallFile(), "-h")
		output, err := helpCommand.CombinedOutput()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(string(output), fmt.Sprintf("Usage: %s", b.InstallFile())))
	})

}

func fakeSiteCrs(routerAccess bool) (string, error) {
	tempDir, err := os.MkdirTemp("", "fakecrs.*")
	if err != nil {
		return "", err
	}
	ss := &api.SiteState{
		Site: &v1alpha1.Site{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Site",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-site",
			},
		},
		Connectors: map[string]*v1alpha1.Connector{
			"my-backend": {
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "Connector",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-backend",
				},
				Spec: v1alpha1.ConnectorSpec{
					RoutingKey: "my-backend",
					Host:       "127.0.0.1",
					Port:       8080,
				},
			},
		},
		Listeners: map[string]*v1alpha1.Listener{
			"my-listener": {
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "Listener",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-listener",
				},
				Spec: v1alpha1.ListenerSpec{
					RoutingKey: "my-listener",
					Host:       "127.0.0.1",
					Port:       9090,
				},
			},
		},
	}
	if routerAccess {
		ss.RouterAccesses = map[string]*v1alpha1.RouterAccess{
			"default": {
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "RouterAccess",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1alpha1.RouterAccessSpec{
					Roles: []v1alpha1.RouterAccessRole{
						{
							Name: "inter-router",
							Port: 55671,
						},
						{
							Name: "edge",
							Port: 45671,
						},
					},
					BindHost: "127.0.0.1",
					SubjectAlternativeNames: []string{
						"localhost",
						"127.0.0.1",
					},
				},
			},
		}
	}
	siteDir := path.Join(tempDir, ss.Site.ObjectMeta.Name)
	err = os.MkdirAll(siteDir, 0755)
	if err != nil {
		return tempDir, err
	}
	err = api.MarshalSiteState(*ss, siteDir)
	return tempDir, err
}
