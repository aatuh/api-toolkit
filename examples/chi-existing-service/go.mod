module example.com/chi-existing-service

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v3 v3.1.2
	github.com/go-chi/chi/v5 v5.2.5
)

replace github.com/aatuh/api-toolkit/v3 => ../..
