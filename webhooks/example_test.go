package webhooks_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v4/webhooks"
)

func ExampleNewHMACSHA256Verifier() {
	body := []byte(`{"id":"evt_123"}`)
	signer, err := webhooks.NewHMACSHA256Signer(webhooks.HMACSignerConfig{
		Secret: []byte("secret"),
	})
	if err != nil {
		panic(err)
	}
	signature, err := signer.SignWebhook(context.Background(), body)
	if err != nil {
		panic(err)
	}
	verifier, err := webhooks.NewHMACSHA256Verifier(webhooks.HMACConfig{
		Secret:     []byte("secret"),
		HeaderName: "X-Signature",
	})
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhooks", nil)
	req.Header.Set("X-Signature", signature)

	err = verifier.VerifyWebhook(context.Background(), req, body)
	fmt.Println(err == nil)

	// Output:
	// true
}
