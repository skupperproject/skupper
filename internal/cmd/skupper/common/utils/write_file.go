package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(path string, name string, content string) error {

	// Resolve the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fullPath := filepath.Join(homeDir, path)

	completeFilePath := fullPath + "/" + name

	// Create the directories recursively
	err = os.MkdirAll(fullPath, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directories: %s", err)
	}

	file, err := os.Create(completeFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	defer file.Sync()

	fmt.Println("File written to", completeFilePath)

	return nil
}
