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
	"crypto/rand"
	"os"
	"os/user"
	"slices"
	"strings"
)

const alphanumerics = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandomId(length int) string {
	buffer := make([]byte, length)
	rand.Read(buffer)
	max := len(alphanumerics)
	for i := range buffer {
		buffer[i] = alphanumerics[int(buffer[i])%max]
	}
	return string(buffer)
}

func StringifySelector(labels map[string]string) string {
	result := ""
	for k, v := range labels {
		if result != "" {
			result += ","
		}
		result += k
		result += "="
		result += v
	}
	return result
}

func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !slices.Contains(b, v) {
			return false
		}
	}
	return true
}

// DefaultStr returns the first non-empty string
func DefaultStr(values ...string) string {
	if len(values) == 1 {
		return values[0]
	}
	if values[0] != "" {
		return values[0]
	}
	return DefaultStr(values[1:]...)
}

func ReadUsername() string {
	u, err := user.Current()
	if err != nil {
		return DefaultStr(os.Getenv("USER"), os.Getenv("USERNAME"))
	}
	return strings.Join(strings.Fields(u.Username), "")
}
