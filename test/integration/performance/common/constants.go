package common

const (
	ENV_SKUPPER_SITES              = "SKUPPER_SITES"
	ENV_SKUPPER_MAX_FRAME_SIZE     = "SKUPPER_MAX_FRAME_SIZE"
	ENV_SKUPPER_MAX_SESSION_FRAMES = "SKUPPER_MAX_SESSION_FRAMES"
	ENV_SKUPPER_MEMORY             = "SKUPPER_MEMORY"
	ENV_SKUPPER_CPU                = "SKUPPER_CPU"
	ENV_SKUPPER_PERF_TIMEOUT       = "SKUPPER_PERF_TIMEOUT"
	TestPath                       = "./tmp"
)

var (
	OutputPath = TestPath
)
