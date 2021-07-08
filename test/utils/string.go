package utils

// StrDefault helper function that returns value if not empty
// otherwise returns dflt.
func StrDefault(dflt string, values ...string) string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return dflt
}

// StrEmpty returns true if the given string is empty
func StrEmpty(value string) bool {
	if value == "" {
		return true
	}
	return false
}

// StrIn returns true if the given value exists in values
func StrIn(value string, values ...string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// AllStrIn returns true if all values provided exist in slice
func AllStrIn(slice []string, values ...string) bool {
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if !StrIn(v, slice...) {
			return false
		}
	}
	return true
}
