package formatter

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

func PrintKeyValueMap(entries map[string]string) error {
	writer := new(tabwriter.Writer)
	writer.Init(os.Stdout, 8, 8, 0, '\t', 0)
	defer writer.Flush()

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	_, err := fmt.Fprint(writer, "")
	if err != nil {
		return err
	}

	for _, key := range keys {
		_, err := fmt.Fprintf(writer, "\n %s\t%s\t", key, entries[key])
		if err != nil {
			return err
		}
	}

	return nil
}
