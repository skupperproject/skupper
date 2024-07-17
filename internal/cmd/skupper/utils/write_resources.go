package utils

import "os"

func CreateFileWithResources(resources []string, outputFormat string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, resource := range resources {
		file.WriteString(resource)
		if outputFormat == "yaml" {
			file.WriteString("---\n")
		}
	}

	return nil
}
