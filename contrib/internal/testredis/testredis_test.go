package testredis

import "testing"

func TestTestURLFromEnvironmentRejectsUnsafeEndpoints(t *testing.T) {
	t.Setenv(EnvironmentURL, "")
	endpoint, err := testURLFromEnvironment()
	if err != nil || endpoint != "redis://127.0.0.1:56379/15" {
		t.Fatalf("default test endpoint = (%q, %v)", endpoint, err)
	}
	for _, endpoint := range []string{
		"redis://redis.example.test:56379/15",
		"redis://127.0.0.1:6379/15",
		"redis://127.0.0.1:56379/0",
		"redis://password@127.0.0.1:56379/15",
		"rediss://127.0.0.1:56379/15",
	} {
		t.Setenv(EnvironmentURL, endpoint)
		if _, err := testURLFromEnvironment(); err == nil {
			t.Fatalf("unsafe test endpoint %q was accepted", endpoint)
		}
	}
}
