package main

// Pet matches the Pet schema in openapi.json.
type Pet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewPet matches the NewPet request schema.
type NewPet struct {
	Name string `json:"name"`
}
