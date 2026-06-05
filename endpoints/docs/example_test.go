package docs_test

import (
	"fmt"

	docsendpoint "github.com/aatuh/api-toolkit/v3/endpoints/docs"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func ExampleNewWithConfig() {
	manager := docsendpoint.NewWithConfig(ports.DocsConfig{
		Title:      "Widget API",
		Version:    "1.0.0",
		EnableHTML: true,
		EnableJSON: false,
	})

	info := manager.GetInfo()
	fmt.Println(info.Title)
	fmt.Println(info.Version)

	// Output:
	// Widget API
	// 1.0.0
}
