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

type RetryError struct {
	n int
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("still failing after %d retries", e.n)
}

func IsRetryFailure(err error) bool {
	_, ok := err.(*RetryError)
	return ok
}

type ConditionFunc func() (bool, error)

// Retry retries f every interval until after maxRetries.
// The interval won't be affected by how long f takes.
// For example, if interval is 3s, f takes 1s, another f will be called 2s later.
// However, if f takes longer than interval, it will be delayed.
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
			break
		}
		<-tick.C
	}
	return &RetryError{maxRetries}
}

// RetryWithContext retries f every interval until the specified context times out.
func RetryWithContext(ctx context.Context, interval time.Duration, f ConditionFunc) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("the context timeout has been reached")
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
