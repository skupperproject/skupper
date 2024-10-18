/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"fmt"
	"io"
	"os"
	"unicode"

	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/env"
	"github.com/skupperproject/skupper/internal/cmd/skupper/diagnose/log"
)

type Reporter interface {
	// Start reports that an operation or sequence of operations is starting;
	// any operation in progress is ended.
	Start(message string, args ...interface{})

	// Success reports that the last operation succeeded with the specified message.
	Success(message string, args ...interface{})

	// Failure reports that the last operation failed with the specified message.
	Failure(message string, args ...interface{})

	// End the current operation that was previously initiated via Start.
	End()

	// Warning reports a warning message for the last operation.
	Warning(message string, args ...interface{})

	// Error wraps err with the supplied message, reports it as a failure, ends the current operation, and returns the error.
	// If err is nil, does nothing and returns nil.
	Error(err error, message string, args ...interface{}) error
}

type resultType int

const (
	success resultType = iota
	failure
	warning
)

type (
	successType string
	warningType string
	failureType string
)

// status is used to track ongoing status in a CLI, with a nice loading spinner
// when attached to a terminal.
type status struct {
	spinner *Spinner
	status  string
	logger  log.Logger
	// for controlling coloring etc.
	successFormat string
	failureFormat string
	warningFormat string
	// message queue
	messageQueue []interface{}
}

func NewReporter() Reporter {
	var writer io.Writer = os.Stderr
	if env.IsSmartTerminal(writer) {
		writer = NewSpinner(writer)
	}

	s := &status{
		logger:        NewLogger(writer, 0),
		successFormat: " ✓ %s\n",
		failureFormat: " ✗ %s\n",
		warningFormat: " ⚠ %s\n",
		messageQueue:  []interface{}{},
	}

	// if we're using the CLI logger, check for if it has a spinner setup
	// and wire the status to that.
	if l, ok := s.logger.(*Logger); ok {
		if w, ok := l.writer.(*Spinner); ok {
			s.spinner = w
			// use colored success / failure / warning messages.
			s.successFormat = " \x1b[32m✓\x1b[0m %s\n"
			s.failureFormat = " \x1b[31m✗\x1b[0m %s\n"
			s.warningFormat = " \x1b[33m⚠\x1b[0m %s\n"
		}
	}

	return s
}

func (s *status) hasFailureMessages() bool {
	for _, message := range s.messageQueue {
		if _, ok := message.(failureType); ok {
			return true
		}
	}

	return false
}

func (s *status) hasWarningMessages() bool {
	for _, message := range s.messageQueue {
		if _, ok := message.(warningType); ok {
			return true
		}
	}

	return false
}

func (s *status) resultFromMessages() resultType {
	if s.hasFailureMessages() {
		return failure
	}

	if s.hasWarningMessages() {
		return warning
	}

	return success
}

// Start starts a new phase of the status, if attached to a terminal
// there will be a loading spinner with this status.
func (s *status) Start(message string, args ...interface{}) {
	s.End()
	s.status = fmt.Sprintf(message, args...)

	if s.spinner != nil {
		s.spinner.SetSuffix(fmt.Sprintf(" %s ", s.status))
		s.spinner.Start()
	} else {
		s.logger.V(0).Infof(" • %s  ...\n", s.status)
	}
}

// Failure queues up a message, which will be displayed once
// the status ends (using the failure format).
func (s *status) Failure(message string, a ...interface{}) {
	if message == "" {
		return
	}

	if s.status != "" {
		s.messageQueue = append(s.messageQueue, failureType(fmt.Sprintf(message, a...)))
	} else {
		s.logger.V(0).Infof(s.failureFormat, fmt.Sprintf(message, a...))
	}
}

// Success queues up a message, which will be displayed once
// the status ends (using the warning format).
// An empty message will result in the start message being confirmed.
func (s *status) Success(message string, a ...interface{}) {
	if message == "" {
		return
	}

	if s.status != "" {
		s.messageQueue = append(s.messageQueue, successType(fmt.Sprintf(message, a...)))
	} else {
		s.logger.V(0).Infof(s.successFormat, fmt.Sprintf(message, a...))
	}
}

// Warning queues up a message, which will be displayed once
// the status ends (using the warning format).
func (s *status) Warning(message string, a ...interface{}) {
	if message == "" {
		return
	}

	if s.status != "" {
		s.messageQueue = append(s.messageQueue, warningType(fmt.Sprintf(message, a...)))
	} else {
		s.logger.V(0).Infof(s.warningFormat, fmt.Sprintf(message, a...))
	}
}

func (s *status) Error(err error, message string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	if message != "" {
		err = fmt.Errorf("%s: %w", fmt.Sprintf(message, args...), err)
	}

	capitalizeFirst := func(str string) string {
		for i, v := range str {
			return string(unicode.ToUpper(v)) + str[i+1:]
		}

		return ""
	}

	s.Failure(capitalizeFirst(err.Error()))
	s.End()

	return err
}

// End completes the current status, ending any previous spinning and
// marking the status as success or failure.
func (s *status) End() {
	if s.status == "" {
		return
	}

	if s.spinner != nil {
		s.spinner.Stop()
		fmt.Fprint(s.spinner.writer, "\r")
	}

	result := s.resultFromMessages()

	switch result {
	case success:
		s.logger.V(0).Infof(s.successFormat, s.status)
	case failure:
		s.logger.V(0).Infof(s.failureFormat, s.status)
	case warning:
		s.logger.V(0).Infof(s.warningFormat, s.status)
	}

	for _, message := range s.messageQueue {
		switch m := message.(type) {
		case successType:
			s.logger.V(0).Infof(s.successFormat, m)
		case failureType:
			s.logger.V(0).Infof(s.failureFormat, m)
		case warningType:
			s.logger.V(0).Infof(s.warningFormat, m)
		}
	}

	s.status = ""
	s.messageQueue = []interface{}{}
}
