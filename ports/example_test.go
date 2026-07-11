package ports_test

import "github.com/aatuh/api-toolkit/v4/ports"

func ExampleNopLogger() {
	var logger ports.Logger = ports.NopLogger{}
	logger.Info("request completed", "request_id", "req_123")
}
