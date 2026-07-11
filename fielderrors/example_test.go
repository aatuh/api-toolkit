package fielderrors_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
)

func ExampleFieldErrors() {
	errs := fielderrors.FieldErrors{{
		Field:   "name",
		Code:    "required",
		Message: "name is required",
	}}

	fmt.Println(errs.Error())
	fmt.Println(errs.ToMap()["name"])

	// Output:
	// name is required
	// name is required
}
