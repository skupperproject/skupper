package utils

import (
	"regexp"
	"strconv"
	"strings"
)

type Version struct {
	Major     int
	Minor     int
	Patch     int
	Qualifier string
}

func ParseVersion(version string) Version {
	var result Version
	re := regexp.MustCompile(`^v?(\d+)[\.\+\-](\d+)?(\.(\d+))?\W?(.+)?`)
	parts := re.FindStringSubmatch(version)
	if len(parts) > 1 {
		result.Major, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		result.Minor, _ = strconv.Atoi(parts[2])
	}
	if len(parts) > 4 {
		result.Patch, _ = strconv.Atoi(parts[4])
	}
	if len(parts) > 5 {
		result.Qualifier = parts[5]
	}
	return result
}

func (a *Version) MoreRecentThan(b Version) bool {
	if a.Major > b.Major {
		return true
	} else if a.Major < b.Major {
		return false
	}
	// a.Major == b.Major, so look at Minor
	if a.Minor > b.Minor {
		return true
	} else if a.Minor < b.Minor {
		return false
	}
	//a.Minor == b.Minor, so look at Patch
	return a.Patch > b.Patch
}

func (a *Version) LessRecentThan(b Version) bool {
	if a.Major < b.Major {
		return true
	} else if a.Major > b.Major {
		return false
	}
	// a.Major == b.Major, so look at Minor
	if a.Minor < b.Minor {
		return true
	} else if a.Minor > b.Minor {
		return false
	}
	//a.Minor == b.Minor, so look at Patch
	return a.Patch < b.Patch
}

func (a *Version) Equivalent(b Version) bool {
	return a.Major == b.Major && a.Minor == b.Minor && a.Patch == b.Patch
}

func (v *Version) IsUndefined() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Qualifier == ""
}

func EquivalentVersion(a string, b string) bool {
	va := ParseVersion(a)
	vb := ParseVersion(b)
	return va.Equivalent(vb)
}

func LessRecentThanVersion(a string, b string) bool {
	va := ParseVersion(a)
	vb := ParseVersion(b)
	return va.LessRecentThan(vb)
}

func MoreRecentThanVersion(a string, b string) bool {
	va := ParseVersion(a)
	vb := ParseVersion(b)
	return va.MoreRecentThan(vb)
}

func IsValidFor(actual string, minimum string) bool {
	if actual == "" { //assume pre 0.5
		return false
	}
	va := ParseVersion(actual)
	vb := ParseVersion(minimum)
	isModified := strings.Contains(va.Qualifier, "-modified")
	return isModified || va.IsUndefined() || !va.LessRecentThan(vb)
}

func GetVersionTag(imageDescriptor string) string {
	versionTag := ""
	imageDescriptorSlices := strings.Split(imageDescriptor, " ")

	if len(imageDescriptorSlices) > 0 {
		imageSlices := strings.Split(imageDescriptorSlices[0], ":")

		if len(imageSlices) > 1 {
			versionTag = imageSlices[1]
		}
	}

	return versionTag
}
