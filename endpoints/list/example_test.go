package list_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v4/endpoints/list"
)

func ExampleParseListQueryChecked() {
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/widgets?limit=5&filter[status]=active&sort=-created_at",
		nil,
	)

	query, err := list.ParseListQueryChecked(req, list.ListQueryConfig{
		DefaultLimit:   10,
		MaxLimit:       50,
		AllowedFilters: []string{"status"},
		AllowedSorts:   []string{"created_at"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(query.Limit)
	fmt.Println(query.First("status"))
	fmt.Println(query.Sort[0].Field, query.Sort[0].Desc)

	// Output:
	// 5
	// active
	// created_at true
}
