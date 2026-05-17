package asynctest

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v2/async"
)

func TestAssertStoreContract(t *testing.T) {
	AssertStoreContract(t, func(testing.TB) async.Store {
		return &fakeStore{job: async.Job{ID: "job_1", Kind: "widgets.import", TenantID: "org_1"}}
	})
}

type fakeStore struct {
	job       async.Job
	leased    bool
	completed bool
	failed    bool
}

func (s *fakeStore) Lease(context.Context, int) ([]async.Job, error) {
	if s.leased {
		return nil, nil
	}
	s.leased = true
	return []async.Job{s.job}, nil
}

func (s *fakeStore) Complete(context.Context, string) error {
	s.completed = true
	return nil
}

func (s *fakeStore) Fail(context.Context, string, string) error {
	s.failed = true
	return nil
}
