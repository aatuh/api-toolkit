module example.com/chi-existing-service

go 1.25.0

require (
	github.com/aatuh/api-toolkit/v4 v4.0.0
	github.com/go-chi/chi/v5 v5.3.1
)

replace github.com/aatuh/api-toolkit/v4 => ../..
