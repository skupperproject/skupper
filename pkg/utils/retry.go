/*
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

package utils

import (
	"context"
	"fmt"
	"time"
)

type ConditionFunc func() (bool, error)

type CheckedFunc func() error

// Retry retries f every interval until after maxRetries.
//
// The interval won't be affected by how long f takes.
// For example, if interval is 3s, f takes 1s, another f will be called 2s later.
// However, if f takes longer than interval, it will be delayed.
//
// If an error is received from f, fail immediatelly and return that error (no
// further retries).
//
// Keep in mind that the second argument is for max _retries_.  So, with a value
// of 1, f() will run at most 2 times (one try and one _retry_).
func Retry(interval time.Duration, maxRetries int, f ConditionFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for i := 0; ; i++ {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i == maxRetries {
			return fmt.Errorf("still failing after %d retries", i)
		}
		<-tick.C
	}
}

// This is similar to Retry(), but it will not fail immediatelly on errors, and
// if the retries are exausted and f() still failing, it will return f()'s error
func RetryError(interval time.Duration, maxRetries int, f CheckedFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for i := 0; ; i++ {
		err := f()
		if err == nil {
			return nil
		}
		if i == maxRetries {
			return err
		}
		<-tick.C
	}
}

type Result struct {
	Value interface{}
	Error error
}

type ResultFunc func() Result

func TryUntil(maxWindowTime time.Duration, f ResultFunc) (interface{}, error) {

	result := make(chan Result, 1)

	go func() {
		result <- f()
	}()
	select {
	case <-time.After(maxWindowTime):
		return nil, fmt.Errorf("timed out")
	case result := <-result:
		return result.Value, result.Error
	}
}

// RetryWithContext retries f every interval until the specified context times out.
func RetryWithContext(ctx context.Context, interval time.Duration, f ConditionFunc) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return context.DeadlineExceeded
		case <-tick.C:
			r, err := f()
			if err != nil {
				return err
			}
			if r {
				return nil
			}
		}
	}
}
