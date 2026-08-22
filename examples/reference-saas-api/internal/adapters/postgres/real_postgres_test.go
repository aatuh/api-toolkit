package postgres

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/migrator"
	"github.com/aatuh/api-toolkit/contrib/v4/testpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
	"github.com/aatuh/api-toolkit/v4/operations"

	"example.com/reference-saas-api/internal/app"
	"example.com/reference-saas-api/internal/domain"
)

// TestRealPostgresReferenceServicePersistence verifies the generated reference
// service against the same isolated PostgreSQL harness used by supported
// adapters. It covers the generated migration, tenancy, widgets, API keys,
// object metadata, durable imports, and encrypted webhook endpoint paths.
func TestRealPostgresReferenceServicePersistence(t *testing.T) {
	if os.Getenv(testpostgres.EnableEnv) != "1" {
		t.Skipf("requires %s=1; run make supported-adapter-check", testpostgres.EnableEnv)
	}

	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, migrator.Options{MigrationsDirs: []string{referenceMigrationsDir(t)}}); err != nil {
		t.Fatalf("apply reference service migrations: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	tenantID := h.FixtureID("tenant")
	actorID := h.FixtureID("actor")
	tenancy := NewTenancyStore(h.Pool())
	if err := tenancy.CreateOrganization(ctx,
		domain.Organization{ID: tenantID, Name: "Real PostgreSQL fixture", CreatedAt: now, UpdatedAt: now},
		domain.Membership{OrganizationID: tenantID, UserID: actorID, Role: domain.RoleOwner, CreatedAt: now},
	); err != nil {
		t.Fatalf("create generated-service organization: %v", err)
	}
	organizations, err := tenancy.ListOrganizations(ctx, actorID)
	if err != nil || len(organizations) != 1 || organizations[0].ID != tenantID {
		t.Fatalf("list generated-service organizations = %#v, error=%v", organizations, err)
	}

	widgets := NewWidgetStore(h.Pool())
	widget := domain.Widget{ID: h.FixtureID("widget"), TenantID: tenantID, Name: "fixture", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := widgets.Save(ctx, widget); err != nil {
		t.Fatalf("save generated-service widget: %v", err)
	}
	storedWidget, found, err := widgets.Get(ctx, tenantID, widget.ID)
	if err != nil || !found || storedWidget.Name != widget.Name {
		t.Fatalf("get generated-service widget = %#v, found=%t, error=%v", storedWidget, found, err)
	}

	objects := NewObjectStore(h.Pool())
	object := app.Object{TenantID: tenantID, Key: "fixtures/report.json", ContentType: "application/json", Size: 2, CreatedAt: now, UpdatedAt: now}
	if err := objects.SaveObjectMetadata(ctx, object); err != nil {
		t.Fatalf("save generated-service object metadata: %v", err)
	}
	storedObject, found, err := objects.GetObjectMetadata(ctx, tenantID, object.Key)
	if err != nil || !found || storedObject.ContentType != object.ContentType {
		t.Fatalf("get generated-service object metadata = %#v, found=%t, error=%v", storedObject, found, err)
	}

	apiKeys := NewAPIKeyStore(h.Pool())
	keyHash := strings.Repeat("a", 64)
	apiKey := domain.APIKey{ID: h.FixtureID("api-key"), OrganizationID: tenantID, Name: "fixture", Prefix: "atk", Scopes: []string{"widgets:read"}, CreatedAt: now}
	if err := apiKeys.CreateAPIKey(ctx, apiKey, keyHash); err != nil {
		t.Fatalf("create generated-service API key: %v", err)
	}
	storedKey, found, err := apiKeys.GetAPIKeyByHash(ctx, keyHash)
	if err != nil || !found || storedKey.ID != apiKey.ID {
		t.Fatalf("get generated-service API key = %#v, found=%t, error=%v", storedKey, found, err)
	}

	operationStore := NewWidgetImportOperationStore(h.Adapter())
	operationID := h.FixtureID("import-operation")
	if err := operationStore.CreateWidgetImportOperation(ctx, tenantID, operations.Operation[app.WidgetImportResult]{ID: operationID, State: operations.StatePending}); err != nil {
		t.Fatalf("create generated-service import operation: %v", err)
	}
	imports := NewWidgetImportOutbox(h.Adapter(), operationStore)
	eventID := h.FixtureID("import-event")
	if err := imports.EnqueueWidgetImport(ctx, app.WidgetImportEvent{
		ID: eventID, TenantID: tenantID, Kind: "widget.import", OperationID: operationID, Payload: []byte(`{"operation_id":"` + operationID + `"}`),
	}); err != nil {
		t.Fatalf("enqueue generated-service import: %v", err)
	}
	jobs, err := imports.Lease(ctx, 1)
	if err != nil || len(jobs) != 1 || jobs[0].ID != eventID {
		t.Fatalf("lease generated-service import = %#v, error=%v", jobs, err)
	}
	if err := imports.Complete(ctx, eventID); err != nil {
		t.Fatalf("complete generated-service import: %v", err)
	}

	webhooks, err := NewWebhookStore(h.Adapter(), strings.Repeat("k", 32), webhookdelivery.EndpointPolicy{})
	if err != nil {
		t.Fatalf("construct generated-service webhook store: %v", err)
	}
	endpoint := webhookdelivery.Endpoint{
		ID: h.FixtureID("endpoint"), TenantID: tenantID, URL: "https://example.invalid/hooks", Events: []string{"widget.created"}, SigningSecret: []byte("fixture-signing-material"), CreatedAt: now, UpdatedAt: now,
	}
	if err := webhooks.CreateEndpoint(ctx, endpoint); err != nil {
		t.Fatalf("create generated-service webhook endpoint: %v", err)
	}
	storedEndpoint, found, err := webhooks.GetEndpoint(ctx, tenantID, endpoint.ID)
	if err != nil || !found || string(storedEndpoint.SigningSecret) != string(endpoint.SigningSecret) {
		t.Fatalf("get generated-service webhook endpoint id=%q found=%t signing_secret_length=%d error=%v", storedEndpoint.ID, found, len(storedEndpoint.SigningSecret), err)
	}
}

func referenceMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate reference service integration test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
}
