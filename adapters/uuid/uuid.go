package uuid

import (
	"github.com/aatuh/api-toolkit/ports"
	"github.com/google/uuid"
)

type UUIDGen struct{}

// NewUUIDGen creates a new Generator backed by github.com/google/uuid.
func NewUUIDGen() ports.IDGen {
	return &UUIDGen{}
}

func (UUIDGen) New() string {
	return uuid.NewString()
}
