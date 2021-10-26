package client

const (
	ValidRfc1123Label                = `^(` + ValidRfc1123LabelKey + `)+=(` + ValidRfc1123LabelValue + `)+(,(` + ValidRfc1123LabelKey + `)+=(` + ValidRfc1123LabelValue + `)+)*$`
	ValidRfc1123LabelKey             = "[a-z0-9]([-._a-z0-9]*[a-z0-9])*"
	ValidRfc1123LabelValue           = "[a-zA-Z0-9]([-._a-zA-Z0-9]*[a-zA-Z0-9])*"
	DefaultSkupperExtraLabels string = ""
)
