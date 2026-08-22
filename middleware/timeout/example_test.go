package timeout_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/timeout"
)

func ExampleNewPropagator() {
	middleware, err := timeout.NewPropagator(timeout.Options{Timeout: time.Second})
	if err != nil {
		panic(err)
	}

	fmt.Println(middleware.Timeout)

	// Output:
	// 1s
}

func ExampleHardTimeout_WrapRoute() {
	hardTimeout, err := timeout.NewHard(timeout.Options{Timeout: time.Second})
	if err != nil {
		panic(err)
	}
	handler, err := hardTimeout.WrapRoute(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), timeout.RouteCapabilities{})
	if err != nil {
		panic(err)
	}
	fmt.Println(handler != nil)

	// Output:
	// true
}

func ExampleRouteCapabilities() {
	capabilities := timeout.RouteCapabilities{
		Streaming:        true,
		ServerSentEvents: true,
		WebSocketUpgrade: true,
		LargeDownload:    true,
		Flusher:          true,
		Hijacker:         true,
		Pusher:           true,
		ReaderFrom:       true,
	}
	fmt.Println(capabilities.ValidateHardTimeout() != nil)

	// Output:
	// true
}

func ExampleHardTimeoutEventHooks() {
	hooks := &timeout.HardTimeoutEventHooks{
		OnHandlerContinuesAfterTimeout: func(timeout.HardTimeoutEvent) {},
	}
	fmt.Println(hooks.OnHandlerContinuesAfterTimeout != nil)

	// Output:
	// true
}
