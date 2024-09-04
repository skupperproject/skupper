package bundle

import (
	_ "embed"
	"os"
	"slices"

	"github.com/skupperproject/skupper/internal/utils"
)

const (
	scriptExit = "\nexit 0\n"
	shellDelim = "\n__TARBALL_CONTENT__\n"
	BundleEnv  = "SKUPPER_BUNDLE"
)

var (
	//go:embed install.sh.template
	installScript string
	//go:embed self_extract.sh.template
	selfExtractPart         string
	allowedBundleStrategies = []string{string(BundleStrategyExtract), string(BundleStrategyTarball)}
)

type BundleGenerator interface {
	Generate(tarball *utils.Tarball, defaultPlatform string) error
}

type BundleStrategy string

var (
	BundleStrategyExtract BundleStrategy = "bundle"
	BundleStrategyTarball BundleStrategy = "tarball"
)

func IsValidBundle(strategy string) bool {
	return slices.Contains(allowedBundleStrategies, strategy)
}

// GetBundleStrategy returns the provided strategy (argument) if it is
// a valid strategy. If not, it returns the SKUPPER_BUNDLE environment
// variable's value if it is valid, or empty otherwise.
func GetBundleStrategy(strategyFlag string) string {
	if IsValidBundle(strategyFlag) {
		return strategyFlag
	}
	if envStrategy := os.Getenv(BundleEnv); IsValidBundle(envStrategy) {
		return envStrategy
	}
	return ""
}
