package uuid

import (
	"github.com/aatuh/api-toolkit/ports"
	"github.com/google/uuid"
)

// UUIDGen generates UUID strings.
//
//revive:disable-next-line:exported
type UUIDGen struct{}

// NewUUIDGen creates a new Generator backed by github.com/google/uuid.
func NewUUIDGen() ports.IDGen {
	return &UUIDGen{}
}

// New returns a new UUID string.
func (UUIDGen) New() string {
	return uuid.NewString()
}
