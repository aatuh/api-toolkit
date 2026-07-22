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
	hardTimeout, err := timeout.NewHard(timeout.Options{
		Timeout: 2 * time.Second,
		EventHooks: &timeout.HardTimeoutEventHooks{
			OnHandlerContinues: func(timeout.HardTimeoutContinuationEvent) {},
		},
	})
	if err != nil {
		panic(err)
	}

	finiteJSON, err := hardTimeout.WrapRoute(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}), timeout.RouteCapabilityFiniteJSON)
	if err != nil {
		panic(err)
	}
	_ = finiteJSON

	capabilities := []timeout.RouteCapabilities{
		timeout.RouteCapabilityStreaming,
		timeout.RouteCapabilityServerSentEvents,
		timeout.RouteCapabilityWebSocketUpgrade,
		timeout.RouteCapabilityLargeDownload,
		timeout.RouteCapabilityFlusher,
		timeout.RouteCapabilityHijacker,
		timeout.RouteCapabilityPusher,
		timeout.RouteCapabilityReaderFrom,
	}
	event := timeout.HardTimeoutContinuationEvent{
		Method:       http.MethodGet,
		Duration:     time.Second,
		Timeout:      2 * time.Second,
		CaptureLimit: 1 << 20,
	}

	fmt.Println(len(capabilities), event.Method)

	// Output:
	// 8 GET
}
