package utils

import (
	"os"
)

func WriteFile(name string, content string) error {

	// Open a new file, if it doesn't exist it will be created
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	err = file.Sync()
	if err != nil {
		return err
	}

	return nil
}
