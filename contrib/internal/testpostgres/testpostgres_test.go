package testpostgres

import "testing"

const validTestDSN = "postgres://api_toolkit_test:api_toolkit_test@127.0.0.1:54329/api_toolkit_test?sslmode=disable"

func TestTestDSNFromEnvironmentAcceptsDedicatedLoopbackEndpoint(t *testing.T) {
	t.Setenv(EnvironmentDSN, validTestDSN)
	got, err := testDSNFromEnvironment()
	if err != nil {
		t.Fatalf("testDSNFromEnvironment() error = %v", err)
	}
	if got != validTestDSN {
		t.Fatalf("testDSNFromEnvironment() = %q, want configured DSN", got)
	}
}

func TestTestDSNFromEnvironmentRejectsUnsafeEndpoints(t *testing.T) {
	tests := map[string]string{
		"missing":        "",
		"remote host":    "postgres://api_toolkit_test:pw@db.example:54329/api_toolkit_test?sslmode=disable",
		"standard port":  "postgres://api_toolkit_test:pw@127.0.0.1:5432/api_toolkit_test?sslmode=disable",
		"wrong user":     "postgres://developer:pw@127.0.0.1:54329/api_toolkit_test?sslmode=disable",
		"wrong database": "postgres://api_toolkit_test:pw@127.0.0.1:54329/application?sslmode=disable",
		"TLS omitted":    "postgres://api_toolkit_test:pw@127.0.0.1:54329/api_toolkit_test",
	}
	for name, dsn := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(EnvironmentDSN, dsn)
			if _, err := testDSNFromEnvironment(); err == nil {
				t.Fatal("testDSNFromEnvironment() accepted an unsafe endpoint")
			}
		})
	}
}

func TestSanitizeDSNRemovesCredentialsAndQuery(t *testing.T) {
	got := sanitizeDSN("postgres://api_toolkit_test:very-secret@127.0.0.1:54329/api_toolkit_test?sslmode=disable&application_name=secret")
	if got != "postgres://127.0.0.1:54329/api_toolkit_test" {
		t.Fatalf("sanitizeDSN() = %q", got)
	}
}

func TestGeneratedIdentifierUsesSafePrefix(t *testing.T) {
	got, err := generatedIdentifier("case")
	if err != nil {
		t.Fatalf("generatedIdentifier() error = %v", err)
	}
	if len(got) != len("case_")+16 || got[:5] != "case_" {
		t.Fatalf("generatedIdentifier() = %q", got)
	}
}
