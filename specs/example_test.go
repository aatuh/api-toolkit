package specs_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/specs"
)

func ExampleSchemaRef() {
	schema := specs.SchemaRef("#/components/schemas/Widget")

	fmt.Println(schema["$ref"])

	// Output:
	// #/components/schemas/Widget
}
