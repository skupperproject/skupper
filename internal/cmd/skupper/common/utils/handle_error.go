package utils

import (
	"fmt"
	"syscall"
)

type ErrorType int

const (
	GenericError    ErrorType = 1
	ValidationError ErrorType = 2
)

func HandleError(errType ErrorType, err error) {
	if err != nil {
		fmt.Println(err)
		syscall.Exit(int(errType))
	}
}
