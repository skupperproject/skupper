package nonkube

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestCmdDebug_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             common.CommandDebugFlags
		cobraGenericFlags map[string]string
		setupSite         bool
		expectedError     string
	}

	testTable := []test{
		{
			name:          "too many args",
			flags:         common.CommandDebugFlags{},
			args:          []string{"test", "not-valid"},
			setupSite:     true,
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "empty name",
			flags:         common.CommandDebugFlags{},
			args:          []string{""},
			setupSite:     true,
			expectedError: "filename must not be empty",
		},
		{
			name:          "invalid name",
			flags:         common.CommandDebugFlags{},
			args:          []string{"!Bad"},
			setupSite:     true,
			expectedError: "filename is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name:          "no site exists",
			flags:         common.CommandDebugFlags{},
			args:          []string{"test"},
			setupSite:     false,
			expectedError: "no skupper site found in namespace",
		},
		{
			name:      "ok with name",
			flags:     common.CommandDebugFlags{},
			args:      []string{"test"},
			setupSite: true,
		},
		{
			name:      "ok default name",
			flags:     common.CommandDebugFlags{},
			args:      []string{},
			setupSite: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			// Setup fresh temp directory for each test
			if os.Getuid() == 0 {
				api.DefaultRootDataHome = t.TempDir()
			} else {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}

			// Setup namespace
			namespace := "test"
			command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			command.namespace = namespace

			// Setup site if needed
			if test.setupSite {
				siteHandler := fs.NewSiteHandler(namespace)
				siteResource := v2alpha1.Site{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "Site",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-site",
						Namespace: namespace,
					},
				}
				ipath := api.GetInternalOutputPath(namespace, api.InputSiteStatePath)
				rpath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
				content, _ := siteHandler.EncodeToYaml(siteResource)
				siteHandler.WriteFile(ipath, "test-site.yaml", content, common.Sites)
				siteHandler.WriteFile(rpath, "test-site.yaml", content, common.Sites)
			}

			// Setup platform
			internalPath := api.GetInternalOutputPath(namespace, api.InternalBasePath)
			os.MkdirAll(internalPath, 0755)
			platformData := map[string]string{"platform": "podman"}
			platformYaml, _ := yaml.Marshal(platformData)
			os.WriteFile(filepath.Join(internalPath, "platform.yaml"), platformYaml, 0644)

			command.Flags = &test.flags
			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
			if test.name == "ok default name" {
				assert.Check(t, command.fileName == "")
			}
		})
	}
}

func TestCmdDebug_InputToOptions(t *testing.T) {
	type test struct {
		name      string
		namespace string
		filename  string
	}

	testTable := []test{
		{
			name:      "default namespace",
			namespace: "",
			filename:  "",
		},
		{
			name:      "with namespace",
			namespace: "test",
			filename:  "dump",
		},
		{
			name:      "custom name",
			namespace: "prod",
			filename:  "debug-info",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			command.namespace = test.namespace
			command.fileName = test.filename

			expectedNs := test.namespace
			if expectedNs == "" {
				expectedNs = "default"
			}
			expectedFileName := test.filename
			if expectedFileName == "" {
				expectedFileName = "skupper-dump"
			}

			command.InputToOptions()

			assert.Check(t, command.namespace == expectedNs)
			assert.Check(t, strings.HasPrefix(command.fileName, expectedFileName+"-"+expectedNs+"-"))
			// Check that filename has timestamp format
			parts := strings.Split(command.fileName, "-")
			assert.Check(t, len(parts) >= 3)
		})
	}
}

func TestCmdDebug_Run(t *testing.T) {
	// This test creates a real tarball and attempts to collect system information.
	// It may fail cleanup due to container storage permissions, which is expected.
	// Run with -short to skip this test.
	if testing.Short() {
		t.Skip("skipping Run test in short mode")
	}

	t.Cleanup(func() {
		// Ignore cleanup errors from container storage
		// This is expected when containers create overlay mounts
	})

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}

	namespace := "test"

	// Create site and platform setup
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-site",
			Namespace: namespace,
		},
	}

	command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = namespace
	command.fileName = filepath.Join(t.TempDir(), "test-dump")
	command.platform = "linux" // Use linux to avoid container/systemd operations in test

	// Setup paths
	ipath := api.GetInternalOutputPath(namespace, api.InputSiteStatePath)
	rpath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	internalPath := api.GetInternalOutputPath(namespace, api.InternalBasePath)

	// Create platform file
	os.MkdirAll(internalPath, 0755)
	platformData := map[string]string{"platform": "linux"}
	platformYaml, _ := yaml.Marshal(platformData)
	os.WriteFile(filepath.Join(internalPath, "platform.yaml"), platformYaml, 0644)

	// Create site
	siteHandler := fs.NewSiteHandler(namespace)
	content, _ := siteHandler.EncodeToYaml(siteResource)
	siteHandler.WriteFile(ipath, "test-site.yaml", content, common.Sites)
	siteHandler.WriteFile(rpath, "test-site.yaml", content, common.Sites)

	// Run the command
	err := command.Run()
	assert.NilError(t, err)

	// Verify tarball was created
	dumpFile := command.fileName + ".tar.gz"
	_, err = os.Stat(dumpFile)
	assert.NilError(t, err, "dump file should exist")

	// Verify tarball contents
	f, err := os.Open(dumpFile)
	assert.NilError(t, err)
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	assert.NilError(t, err)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	filesFound := make(map[string]bool)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		filesFound[header.Name] = true
		t.Logf("Found file in tarball: %s", header.Name)
	}

	// Check for expected files - now collected in both runtime and input subdirectories
	expectedPaths := []string{
		"/site-namespace/resources/runtime/Site-test-site.yaml",
		"/site-namespace/resources/runtime/Site-test-site.yaml.txt",
		"/site-namespace/resources/input/Site-test-site.yaml",
		"/site-namespace/resources/input/Site-test-site.yaml.txt",
	}

	for _, path := range expectedPaths {
		if !filesFound[path] {
			t.Errorf("Expected file %s not found in tarball", path)
		}
	}

	// Cleanup
	os.Remove(dumpFile)
}

func TestCmdDebug_ValidateInput_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}

	namespace := "integration-test"

	// Setup complete environment
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "integration-site",
			Namespace: namespace,
		},
	}

	command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = namespace

	// Setup site
	siteHandler := fs.NewSiteHandler(namespace)
	ipath := api.GetInternalOutputPath(namespace, api.InputSiteStatePath)
	rpath := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)
	content, _ := siteHandler.EncodeToYaml(siteResource)
	siteHandler.WriteFile(ipath, "integration-site.yaml", content, common.Sites)
	siteHandler.WriteFile(rpath, "integration-site.yaml", content, common.Sites)

	// Setup platform
	internalPath := api.GetInternalOutputPath(namespace, api.InternalBasePath)
	os.MkdirAll(internalPath, 0755)
	platformData := map[string]string{"platform": "linux"}
	platformYaml, _ := yaml.Marshal(platformData)
	os.WriteFile(filepath.Join(internalPath, "platform.yaml"), platformYaml, 0644)

	// Test validation
	err := command.ValidateInput([]string{"integration-dump"})
	assert.NilError(t, err)
	assert.Check(t, command.fileName == "integration-dump")
	assert.Check(t, command.platform == "linux")
}
