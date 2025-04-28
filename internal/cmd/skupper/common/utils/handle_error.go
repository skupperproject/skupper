package utils

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
)

type ErrorType int

const (
	GenericError    ErrorType = 1
	ValidationError ErrorType = 2
)
const (
	CrdErr     string = "the server could not find the requested resource "
	CrdHelpErr string = "The Skupper CRDs are not yet installed. To install them, run\n\"kubectl apply -f https://skupper.io/v2/install.yaml\""
)

func HandleError(errType ErrorType, err error) {
	if err != nil {
		fmt.Println(err)
		syscall.Exit(int(errType))
	}
}

func HandleMissingCrds(err error) error {
	if err != nil {
		errMsg := strings.Split(err.Error(), "(")
		if strings.Compare(errMsg[0], CrdErr) == 0 {
			err = errors.New(CrdHelpErr)
		}
	}
	return err
}
