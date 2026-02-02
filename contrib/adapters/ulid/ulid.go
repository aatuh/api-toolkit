package ulid

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/aatuh/api-toolkit/ports"
)

// ULIDGen generates ULID strings.
//
//revive:disable-next-line:exported
type ULIDGen struct{}

// NewULIDGen creates a new ULID generator that implements ports.IDGen.
func NewULIDGen() ports.IDGen {
	return &ULIDGen{}
}

// New returns a new ULID string.
func (ULIDGen) New() string {
	t := time.Now().UTC()
	return ulid.MustNew(ulid.Timestamp(t), rand.Reader).String()
}
