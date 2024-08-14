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

type Record interface {
	GetStartTime() uint64
	GetEndTime() uint64
}

var (
	_ ResponseSetter[SiteRecord]           = (*SiteResponse)(nil)
	_ CollectionResponseSetter[SiteRecord] = (*SiteListResponse)(nil)
	_ Record                               = (*SiteRecord)(nil)
)
