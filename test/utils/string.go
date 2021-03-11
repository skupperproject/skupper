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
