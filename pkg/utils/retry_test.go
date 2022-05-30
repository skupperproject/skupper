package utils

import (
	"fmt"
	"testing"
	"time"
)

//     f() return:    Retry()
//
// n   ok     err     maxRetries  return (error)
// --  ----- -----    ----------- --------------
// 1   true,   nil    no          nil
// 2   true,  !nil    no          err from f()
// 3   false,  nil    no          retry
// 4   false, !nil    no          err from f()
//
// 5   true,   nil    yes         nil
// 6   true,  !nil    yes         err from f()
// 7   false,  nil    yes         RetryError
// 8   false, !nil    yes         err from f()
//
// Or:
//
// - If function produces an error, fail immediatelly with that error
// - Else, if ok is true, return nil and succeed
// - Otherwise:
//   - if before maximum retry: retry
//   - if on maximum retry: return a Retry error and fail

type RetryTestItem struct {
	// These configure what the f() function will respond
	err        error
	okOnTry    int // OkOnTry = 1 make it ok right away; OkOnTry = 0 will never be ok
	errorOnTry int
	nilOnTry   int
	// This configures Retry itself
	maxRetries int
	// And those are what we're expecting the actual result to look like
	expectedTries    int
	expectedResponse error

	// these change the normal response of f() until a specific try
}

func TestRetry(t *testing.T) {

	testTable := []RetryTestItem{
		{ // #1
			okOnTry:          1,
			err:              nil,
			maxRetries:       3,
			expectedTries:    1,
			expectedResponse: nil,
		}, { // #2
			okOnTry:          1,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    1,
			expectedResponse: fmt.Errorf("app error"),
		}, { // #3, #7
			okOnTry:          0,
			err:              nil,
			maxRetries:       3,
			expectedTries:    4,
			expectedResponse: fmt.Errorf("still failing after 3 retries"),
		}, { // #4
			okOnTry:          0,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    1,
			expectedResponse: fmt.Errorf("app error"),
		}, { // #3, #1
			okOnTry:          2,
			err:              nil,
			maxRetries:       3,
			expectedTries:    2,
			expectedResponse: nil,
		}, { // #3, #2
			okOnTry:          2,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    2,
			expectedResponse: fmt.Errorf("app error"),
			errorOnTry:       2,
		}, { // #3, #4
			okOnTry:          0,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    2,
			expectedResponse: fmt.Errorf("app error"),
			errorOnTry:       2,
		}, { // #3, #5
			okOnTry:          4,
			err:              nil,
			maxRetries:       3,
			expectedTries:    4,
			expectedResponse: nil,
		}, { // #3, #6
			okOnTry:          4,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    4,
			expectedResponse: fmt.Errorf("app error"),
			errorOnTry:       4,
		}, { // #3, #8
			okOnTry:          0,
			err:              fmt.Errorf("app error"),
			maxRetries:       3,
			expectedTries:    4,
			expectedResponse: fmt.Errorf("app error"),
			errorOnTry:       4,
		}, {
			okOnTry:          1,
			err:              nil,
			maxRetries:       -1,
			expectedTries:    0,
			expectedResponse: fmt.Errorf("maxRetries (%d) should be > 0", -1),
		}, {
			okOnTry:          1,
			err:              nil,
			maxRetries:       0,
			expectedTries:    0,
			expectedResponse: fmt.Errorf("maxRetries (%d) should be > 0", 0),
		},
	}

	for _, item := range testTable {
		name := fmt.Sprintf("okOnTry:%v err:%v expectedTries:%v maxRetries:%v errorOnTry:%v nilOnTry: %v",
			item.okOnTry, item.err, item.expectedTries, item.maxRetries, item.errorOnTry, item.nilOnTry)

		var currentTry int
		t.Run(name, func(t *testing.T) {

			retryErr := Retry(time.Microsecond, item.maxRetries, func() (ok bool, err error) {
				currentTry++
				if currentTry > item.maxRetries+1 {
					// This is a protection for infinite loops
					t.Fatalf("Retry %v > maxRetries %v + 1", currentTry, item.maxRetries)
				}

				ok = item.okOnTry > 0 && currentTry >= item.okOnTry

				if item.errorOnTry > 0 {
					if currentTry >= item.errorOnTry {
						err = item.err
					} else {
						err = nil
					}
				} else {
					err = item.err
				}

				if item.nilOnTry > 0 && currentTry >= item.nilOnTry {
					err = nil
				}

				return

			})

			if item.expectedResponse != nil {
				if retryErr != nil {
					if retryErr.Error() != item.expectedResponse.Error() {
						t.Error("Received error:", retryErr)
					}
				} else {
					t.Error("Received error:", retryErr)
				}
			} else {
				if retryErr != nil {
					t.Error("Received error:", retryErr)
				}
			}

			if currentTry != item.expectedTries {
				t.Errorf("%v != %v", currentTry, item.expectedTries)
			}

		})
	}

}

type TestRetryErrorItem struct {
	workOnTry     int
	expectedTries int
	maxRetries    int
	expectSuccess bool
}

func TestRetryError(t *testing.T) {
	testTable := []TestRetryErrorItem{
		{
			workOnTry:     1,
			expectedTries: 1,
			maxRetries:    3,
			expectSuccess: true,
		}, {
			workOnTry:     2,
			expectedTries: 2,
			maxRetries:    3,
			expectSuccess: true,
		}, {
			workOnTry:     4,
			expectedTries: 4,
			maxRetries:    3,
			expectSuccess: true,
		}, {
			workOnTry:     5,
			expectedTries: 4,
			maxRetries:    3,
			expectSuccess: false,
		},
	}

	for _, item := range testTable {
		name := fmt.Sprintf("workOnTry: %v expectedTries: %v maxRetries: %v expectSuccess: %v",
			item.workOnTry, item.expectedTries, item.maxRetries, item.expectSuccess)
		t.Run(name, func(t *testing.T) {
			var currentTry int

			resp := RetryError(time.Microsecond, item.maxRetries, func() (err error) {
				currentTry++
				if currentTry >= item.workOnTry {
					return nil
				}
				return fmt.Errorf("Still not working")
			})

			if item.expectSuccess != (resp == nil) {
				t.Errorf("Received error: %v", resp)
			}

			if item.expectedTries != currentTry {
				t.Errorf("Returned in %d tries", currentTry)
			}

		})
	}

}

type TestTryUntilItem struct {
	workOnSecond  time.Duration
	funcError     error
	funcValue     interface{}
	maxDuration   time.Duration
	expectTimeout bool
}

func TestTryUntil(t *testing.T) {
	testTable := []TestTryUntilItem{
		{
			workOnSecond:  time.Second,
			funcError:     nil,
			funcValue:     []string{"first", "second", "third"},
			maxDuration:   5 * time.Second,
			expectTimeout: false,
		},
		{
			workOnSecond:  4 * time.Second,
			funcError:     nil,
			funcValue:     5,
			maxDuration:   5 * time.Second,
			expectTimeout: false,
		},
		{
			workOnSecond:  time.Second,
			funcError:     fmt.Errorf("function is not working"),
			funcValue:     nil,
			maxDuration:   5 * time.Second,
			expectTimeout: false,
		},
		{
			workOnSecond:  30 * time.Second,
			funcError:     nil,
			funcValue:     nil,
			maxDuration:   5 * time.Second,
			expectTimeout: true,
		},
	}

	for _, item := range testTable {
		name := fmt.Sprintf("workOnSecond: %v maxDuration: %v expectTimeout: %v",
			item.workOnSecond, item.maxDuration, item.expectTimeout)
		t.Run(name, func(t *testing.T) {

			resp, err := TryUntil(item.maxDuration, func() Result {
				time.Sleep(item.workOnSecond)
				return Result{
					Value: item.funcValue,
					Error: item.funcError,
				}
			})

			fmt.Printf("result: %v", resp)
			fmt.Println()

			if item.expectTimeout && err.Error() != "timed out" {
				t.Errorf("It was expected a timeout but it did not happen")
			}

			if item.funcValue != nil && resp == nil {
				t.Errorf("It was expected to receive a value")
			}

			if !item.expectTimeout && item.funcError != err {
				t.Errorf("Received wrong error: %s", err)
			}

		})
	}

}
