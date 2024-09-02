package utils

import (
	"fmt"
	"syscall"
)

const (
	GenericError    = 1
	ValidationError = 2
)

func HandleError(err error) {
	if err != nil {
		fmt.Println(err)
		syscall.Exit(GenericError)
	}
}

func HandleErrorList(errList []error) {
	if errList != nil && len(errList) > 0 {
		for _, err := range errList {
			fmt.Println(err)
		}

		syscall.Exit(ValidationError)
	}
}

func ErrorsToMessages(errs []error) []string {
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return messages
}
