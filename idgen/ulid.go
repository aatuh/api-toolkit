package idgen

import (
	"crypto/rand"
	"time"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/oklog/ulid/v2"
)

type ULIDGen struct{}

func (ULIDGen) New() string {
	t := time.Now().UTC()
	return ulid.MustNew(ulid.Timestamp(t), rand.Reader).String()
}

// NewULIDGen creates a new ULID generator that implements ports.IDGen.
func NewULIDGen() ports.IDGen {
	return &ULIDGen{}
}
