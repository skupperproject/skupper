package bundle

import (
	_ "embed"

	"github.com/skupperproject/skupper/internal/utils"
)

const (
	scriptExit = "\nexit 0\n"
	shellDelim = "\n__TARBALL_CONTENT__\n"
)

var (
	//go:embed install.sh.template
	installScript string
	//go:embed self_extract.sh.template
	selfExtractPart string
)

type BundleGenerator interface {
	Generate(tarball *utils.Tarball) error
}
