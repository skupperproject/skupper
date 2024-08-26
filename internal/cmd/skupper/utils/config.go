package utils

import "time"

type RetryProfile struct {
	MinimumTimeout  time.Duration
	MinimumInterval time.Duration
}

var DefaultRetryProfile = RetryProfile{
	MinimumTimeout:  30 * time.Second,
	MinimumInterval: 1 * time.Second,
}

var TestRetryProfile = RetryProfile{
	MinimumTimeout:  1 * time.Second,
	MinimumInterval: 1 * time.Millisecond,
}

var selectedProfile = DefaultRetryProfile

func GetConfiguredRetryProfile() RetryProfile { return selectedProfile }
func SetRetryProfile(profile RetryProfile)    { selectedProfile = profile }
