package httpcache_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v4/httpcache"
)

func ExampleEvaluateRead() {
	etag := httpcache.StrongETag("widgets-v1")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets", nil)
	req.Header.Set("If-None-Match", etag.String())

	decision := httpcache.EvaluateRead(req, httpcache.Validators{ETag: etag})

	fmt.Println(decision.NotModified)
	fmt.Println(decision.Status)

	// Output:
	// true
	// 304
}
