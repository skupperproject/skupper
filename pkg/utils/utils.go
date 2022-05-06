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
	"io"
	"os"
	"regexp"
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

// LabelToMap expects label string to be a comma separated
// list of key and value pairs delimited by equals.
func LabelToMap(label string) map[string]string {
	m := map[string]string{}
	labels := strings.Split(label, ",")
	for _, l := range labels {
		if !strings.Contains(l, "=") {
			continue
		}
		entry := strings.Split(l, "=")
		m[entry[0]] = entry[1]
	}
	return m
}

func StringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func StringSliceEndsWith(s []string, e string) bool {
	for _, a := range s {
		if strings.HasSuffix(a, e) {
			return true
		}
	}
	return false
}

func RegexpStringSliceContains(s []string, e string) bool {
	for _, re := range s {
		match, err := regexp.Match(re, []byte(e))
		if err == nil && match {
			return true
		}
	}
	return false
}

func IntSliceContains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func IsDirEmpty(name string) (bool, error) {
	file, err := os.Open(name)

	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = file.Readdir(1)

	if err == io.EOF {
		return true, nil
	}

	return false, err
}

func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !StringSliceContains(b, v) {
			return false
		}
	}
	return true
}
