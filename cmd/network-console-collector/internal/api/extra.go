// extra hand written stuffs to make working with generated code easier
package api

import "strings"

type ResponseSetter[T any] interface {
	SetResults(T)
}

type CollectionResponseSetter[T any] interface {
	ResponseSetter[[]T]
	SetCount(int64)
	SetTimeRangeCount(int64)
}

type Record interface {
	GetStartTime() uint64
	GetEndTime() uint64
}

var (
	_ ResponseSetter[SiteRecord]           = (*SiteResponse)(nil)
	_ CollectionResponseSetter[SiteRecord] = (*SiteListResponse)(nil)
	_ Record                               = (*SiteRecord)(nil)
)

type AtmarkDelimitedString string

func NewAtmarkDelimitedString(parts ...string) AtmarkDelimitedString {
	return AtmarkDelimitedString(strings.Join(parts, "@"))
}

func (a AtmarkDelimitedString) Parts() []string {
	return strings.Split(string(a), "@")
}
