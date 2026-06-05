package jwt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestMiddlewareConcurrentTokenValidationUsesSharedKeyfuncSafely(t *testing.T) {
	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()
	token := signToken(t, jwt.SigningMethodRS256, baseClaims(now), privateKey, "test-kid")
	mw := &Middleware{
		cfg: Config{
			Issuer:   "https://issuer.example",
			Audience: "example",
		},
		jwks:        kf,
		enabled:     true,
		log:         ports.NopLogger{},
		allowedAlgs: []string{"RS256"},
		claimReq: claimRequirements{
			requireSubject:    true,
			requireExpiration: true,
		},
	}

	errs := make(chan error, 32)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject, ok := SubjectFromContext(r.Context())
		if !ok || subject.UserID != "user" {
			errs <- fmt.Errorf("subject = %#v, %v", subject, ok)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				errs <- fmt.Errorf("request %d status = %d body=%s", i, rec.Code, rec.Body.String())
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
