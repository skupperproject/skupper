package utils

import (
	"fmt"
	"syscall"
)

func HandleError(err error) {
	if err != nil {
		fmt.Println(err)
		syscall.Exit(0)
	}
}

func HandleErrorList(errList []error) {
	if errList != nil && len(errList) > 0 {
		for _, err := range errList {
			fmt.Println(err)
		}

		syscall.Exit(0)
	}
}

func ErrorsToMessages(errs []error) []string {
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return messages
}
