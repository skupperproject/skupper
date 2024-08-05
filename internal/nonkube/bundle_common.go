package nonkube

import (
	_ "embed"
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
	Generate(tarball *Tarball) error
}
