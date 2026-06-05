package queryparams_test

import (
	"fmt"
	"net/url"

	"github.com/aatuh/api-toolkit/v3/queryparams"
)

func ExampleParseSort() {
	sort, err := queryparams.ParseSort(url.Values{
		"sort": []string{"name,-created_at"},
	}, queryparams.SortConfig{AllowedFields: []string{"name", "created_at"}})
	if err != nil {
		panic(err)
	}

	fmt.Println(sort.Fields[0].Name, sort.Fields[0].Desc)
	fmt.Println(sort.Fields[1].Name, sort.Fields[1].Desc)

	// Output:
	// name false
	// created_at true
}
