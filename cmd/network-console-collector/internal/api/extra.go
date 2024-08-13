// extra hand written stuffs to make working with generated code easier
package api

type ResponseSetter[T any] interface {
	SetResults(T)
}

type CollectionResponseSetter[T any] interface {
	ResponseSetter[[]T]
	SetCount(int64)
	SetTimeRangeCount(int64)
}

var (
	_ ResponseSetter[SiteRecord]           = (*SiteResponse)(nil)
	_ CollectionResponseSetter[SiteRecord] = (*SiteListResponse)(nil)
)
