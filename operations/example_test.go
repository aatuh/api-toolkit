package operations_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/operations"
)

func ExampleTransitionOperation() {
	operation := operations.Operation[string]{ID: "op_123", State: operations.StatePending}
	next, err := operations.TransitionOperation(operation, operations.TransitionConfig[string]{
		To: operations.StateRunning,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(next.State)

	// Output:
	// running
}
