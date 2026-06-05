package apitest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v3/apitest"
	"github.com/aatuh/api-toolkit/v3/httpx"
)

func ExampleAssertJSON() {
	recorder := httptest.NewRecorder()
	httpx.WriteJSON(recorder, http.StatusOK, map[string]string{"status": "ok"})

	var t testing.T
	apitest.AssertJSON(&t, recorder, map[string]string{"status": "ok"})
}
