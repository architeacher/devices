package shared

import (
	"fmt"
	"net/http"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
)

const (
	HeaderCacheStatus   = "Cache-Status"
	HeaderCacheKey      = "Cache-Key"
	HeaderCacheTTL      = "Cache-TTL"
	HeaderCacheControl  = "Cache-Control"
	HeaderETag          = "ETag"
	HeaderIfNoneMatch   = "If-None-Match"
	HeaderLastModified  = "Last-Modified"
	HeaderVary          = "Vary"
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	HeaderAccept        = "Accept"
)

// SetCacheHeaders sets HTTP cache-related headers based on cache result.
func SetCacheHeaders(w http.ResponseWriter, status ports.CacheStatus, key string, ttl time.Duration, maxAge, staleWhileRevalidate uint) {
	w.Header().Set(HeaderCacheStatus, string(status))

	if key != "" {
		w.Header().Set(HeaderCacheKey, key)
	}

	if ttl > 0 {
		w.Header().Set(HeaderCacheTTL, fmt.Sprintf("%d", int64(ttl.Seconds())))
	}

	cacheControl := fmt.Sprintf("private, max-age=%d", maxAge)
	if staleWhileRevalidate > 0 {
		cacheControl = fmt.Sprintf("%s, stale-while-revalidate=%d", cacheControl, staleWhileRevalidate)
	}
	w.Header().Set(HeaderCacheControl, cacheControl)

	w.Header().Set(HeaderVary, fmt.Sprintf("%s, %s", HeaderAccept, HeaderAuthorization))
}

// SetLastModified sets the Last-Modified header.
func SetLastModified(w http.ResponseWriter, t time.Time) {
	w.Header().Set(HeaderLastModified, t.UTC().Format(http.TimeFormat))
}

// SetETagHeader sets the ETag header.
func SetETagHeader(w http.ResponseWriter, etag string) {
	w.Header().Set(HeaderETag, fmt.Sprintf("\"%s\"", etag))
}

// SetWeakETagHeader sets a weak ETag header.
func SetWeakETagHeader(w http.ResponseWriter, etag string) {
	w.Header().Set(HeaderETag, fmt.Sprintf("W/\"%s\"", etag))
}

// GetIfNoneMatch returns the If-None-Match header value.
func GetIfNoneMatch(r *http.Request) string {
	return r.Header.Get(HeaderIfNoneMatch)
}

// ETagMatches checks if the provided ETag matches the If-None-Match header.
func ETagMatches(r *http.Request, etag string) bool {
	ifNoneMatch := GetIfNoneMatch(r)
	if ifNoneMatch == "" {
		return false
	}

	quotedETag := fmt.Sprintf("\"%s\"", etag)
	weakETag := fmt.Sprintf("W/\"%s\"", etag)

	return ifNoneMatch == quotedETag || ifNoneMatch == weakETag || ifNoneMatch == "*"
}
