package version_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/endpoints/version"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func ExampleNewHandler() {
	handler := version.NewHandler(version.Config{
		Info: ports.VersionInfo{Version: "1.2.3", Commit: "abc123", Date: "2026-06-05"},
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/version", nil)
	handler.Handler().ServeHTTP(recorder, req)

	fmt.Println(recorder.Code)

	// Output:
	// 200
}
