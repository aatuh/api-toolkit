package upload_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/upload"
)

func ExampleAllowedContentTypes() {
	allowed := upload.AllowedContentTypes(" Image/PNG ", "application/pdf")

	fmt.Println(allowed[0])
	fmt.Println(upload.MaxFileBytes(1024))

	// Output:
	// image/png
	// 1024
}
