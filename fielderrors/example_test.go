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

func ExampleFieldError_Public() {
	err := fielderrors.FieldError{
		Field:   "postal_code",
		Code:    "invalid",
		Message: "postal code is invalid",
		Public:  true,
	}

	fmt.Println(err.Public)

	// Output:
	// true
}

func ExampleFieldErrors_AllPublic() {
	errs := fielderrors.FieldErrors{{
		Field:   "postal_code",
		Code:    "invalid",
		Message: "postal code is invalid",
		Public:  true,
	}}

	fmt.Println(errs.AllPublic())

	// Output:
	// true
}
