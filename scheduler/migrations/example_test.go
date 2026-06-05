package migrations_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v3/scheduler/migrations"
)

func ExampleMigrations() {
	entries, err := migrations.Migrations.ReadDir(".")
	if err != nil {
		panic(err)
	}

	fmt.Println(len(entries) > 0)

	// Output:
	// true
}
