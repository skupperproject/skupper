package formatter

type PrettyPrinter interface {
	PrintJsonFormat() (string, error)
	PrintYamlFormat() (string, error)
	ChangeFormat()
}
