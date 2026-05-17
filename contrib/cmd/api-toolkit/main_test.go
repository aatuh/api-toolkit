package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var out strings.Builder
	code := run(context.Background(), []string{"version"}, &out, &out)
	if code != 0 {
		t.Fatalf("version exit code = %d output=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "api-toolkit") {
		t.Fatalf("version output = %q", out.String())
	}
	for _, want := range []string{
		"go go",
		"main ",
		"core github.com/aatuh/api-toolkit/v2 ",
		"contrib github.com/aatuh/api-toolkit/contrib/v2 ",
		"build_commit ",
		"build_date ",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("version output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunVersionJSON(t *testing.T) {
	var out strings.Builder
	code := run(context.Background(), []string{"version", "--json"}, &out, &out)
	if code != 0 {
		t.Fatalf("version --json exit code = %d output=%s", code, out.String())
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("decode version json: %v\n%s", err, out.String())
	}
	for _, key := range []string{
		"tool_version",
		"go_version",
		"main_path",
		"main_version",
		"core_version",
		"contrib_version",
		"build_commit",
		"build_date",
	} {
		if strings.TrimSpace(got[key]) == "" {
			t.Fatalf("version json missing %q: %#v", key, got)
		}
	}
}

func TestRunHelpUsageExitsZero(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "top level long", args: []string{"--help"}, want: usageTopLevel},
		{name: "top level short", args: []string{"-h"}, want: usageTopLevel},
		{name: "top level command", args: []string{"help"}, want: usageTopLevel},
		{name: "version help", args: []string{"version", "help"}, want: usageVersion},
		{name: "new group help", args: []string{"new", "help"}, want: usageNew},
		{name: "new service help", args: []string{"new", "service", "help"}, want: usageNew},
		{name: "new service short help", args: []string{"new", "service", "-h"}, want: usageNew},
		{name: "generate group help", args: []string{"generate", "--help"}, want: usageGenerate},
		{name: "generate resource help", args: []string{"generate", "resource", "help"}, want: usageGenerate},
		{name: "contracts group help", args: []string{"contracts", "help"}, want: usageContracts},
		{name: "contracts lint help", args: []string{"contracts", "lint", "--help"}, want: usageContractsLint},
		{name: "contracts diff help", args: []string{"contracts", "diff", "-h"}, want: usageContractsDiff},
		{name: "contracts changelog help", args: []string{"contracts", "changelog", "--help"}, want: usageContractsChangelog},
		{name: "contracts impact help", args: []string{"contracts", "impact", "-h"}, want: usageContractsImpact},
		{name: "clients group help", args: []string{"clients", "help"}, want: usageClients},
		{name: "clients go help", args: []string{"clients", "go", "--help"}, want: usageClientsGo},
		{name: "clients typescript help", args: []string{"clients", "typescript", "--help"}, want: usageClientsTypeScript},
		{name: "ops group help", args: []string{"ops", "help"}, want: usageOps},
		{name: "ops observability help", args: []string{"ops", "observability", "--help"}, want: usageOpsObservability},
		{name: "deploy group help", args: []string{"deploy", "help"}, want: usageDeploy},
		{name: "deploy helm help", args: []string{"deploy", "helm", "--help"}, want: usageDeployHelm},
		{name: "deploy terraform help", args: []string{"deploy", "terraform", "--help"}, want: usageDeployTerraform},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(context.Background(), tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("help exit code = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.want)
			}
			if stderr.String() != "" {
				t.Fatalf("help wrote stderr: %q", stderr.String())
			}
		})
	}
}

func TestRunUnknownCommandsExitTwo(t *testing.T) {
	tests := [][]string{
		{"unknown"},
		{"new", "unknown"},
		{"generate", "unknown"},
		{"contracts", "unknown"},
		{"clients", "unknown"},
		{"ops", "unknown"},
		{"deploy", "unknown"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(context.Background(), args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.String() == "" {
				t.Fatal("unknown command should explain usage or unknown command")
			}
		})
	}
}

func TestScaffoldDependencyVersionUsesInstalledSemver(t *testing.T) {
	tests := []struct {
		name string
		info versionMetadata
		want string
	}{
		{
			name: "contrib release",
			info: versionMetadata{ContribVersion: "v2.3.4", CoreVersion: "v2.0.0"},
			want: "v2.3.4",
		},
		{
			name: "main release",
			info: versionMetadata{MainVersion: "v2.4.0"},
			want: "v2.4.0",
		},
		{
			name: "development fallback",
			info: versionMetadata{MainVersion: "dev", CoreVersion: "local", ContribVersion: "unknown"},
			want: defaultScaffoldModuleVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaffoldDependencyVersion(tt.info); got != tt.want {
				t.Fatalf("scaffoldDependencyVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateServiceUsesConfiguredToolkitVersion(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "service")
	if err := generateService(scaffoldConfig{
		Module:         "example.com/my-api",
		Dir:            serviceDir,
		Profile:        scaffoldProfileSaaSAPIFull,
		AuthMode:       scaffoldAuthAPIKey,
		ToolkitVersion: "v2.3.4",
	}); err != nil {
		t.Fatalf("generate service: %v", err)
	}
	generatedMod, err := os.ReadFile(filepath.Join(serviceDir, "go.mod"))
	if err != nil {
		t.Fatalf("read generated go.mod: %v", err)
	}
	for _, want := range []string{
		"github.com/aatuh/api-toolkit/v2 v2.3.4",
		"github.com/aatuh/api-toolkit/contrib/v2 v2.3.4",
	} {
		if !strings.Contains(string(generatedMod), want) {
			t.Fatalf("generated go.mod missing %q:\n%s", want, generatedMod)
		}
	}
	if strings.Contains(string(generatedMod), "v2.1.0") {
		t.Fatalf("generated go.mod kept stale default version:\n%s", generatedMod)
	}
}

func TestClientsGoGeneratesBuildableClient(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		},
		"/widgets/{id}": {
			"get": {
				"operationId": "getWidget",
				"parameters": [
					{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
					{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string"}}
				],
				"responses": {
					"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}},
					"404": {"description": "not found", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)
	clientDir := filepath.Join(tmp, "client")
	var out strings.Builder
	code := run(context.Background(), []string{
		"clients", "go",
		"--openapi", specPath,
		"--out", clientDir,
		"--package", "apiclient",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("clients go failed: %s", out.String())
	}
	generated, err := os.ReadFile(filepath.Join(clientDir, "client.go"))
	if err != nil {
		t.Fatalf("read generated client.go: %v", err)
	}
	for _, want := range []string{
		"package apiclient",
		"type Client struct",
		"func WithAPIKey",
		"func WithBearerToken",
		"func (c *Client) CreateWidget",
		"func (c *Client) GetWidget",
		"type Problem struct",
		"type Error struct",
		"PathParam(\"id\",",
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated client.go missing %q:\n%s", want, generated)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/clienttest\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatalf("write temp go.mod: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = tmp
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated client package should build:\n%s\nerror: %v", output, err)
	}
}

func TestClientsGoGeneratesTypedClient(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	spec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			},
			"schemas": {
				"Problem": {
					"type": "object",
					"properties": {
						"type": {"type": "string"},
						"title": {"type": "string"},
						"status": {"type": "integer"},
						"detail": {"type": "string"}
					}
				},
				"Widget": {
					"type": "object",
					"required": ["id", "name", "version"],
					"properties": {
						"id": {"type": "string"},
						"name": {"type": "string"},
						"version": {"type": "integer", "format": "int64"},
						"next_cursor": {"type": "string", "nullable": true},
						"tags": {"type": "array", "items": {"type": "string"}}
					}
				},
				"WidgetCreateRequest": {
					"type": "object",
					"required": ["name"],
					"properties": {
						"name": {"type": "string"}
					}
				}
			}
		},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"requestBody": {
						"required": true,
						"content": {"application/json": {"schema": {"$ref": "#/components/schemas/WidgetCreateRequest"}}}
					},
					"responses": {
						"201": {"description": "created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			},
			"/widgets/{id}": {
				"get": {
					"operationId": "getWidget",
					"parameters": [
						{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
						{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string"}},
						{"name": "include_deleted", "in": "query", "required": false, "schema": {"type": "boolean"}}
					],
					"responses": {
						"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}},
						"404": {"description": "not found", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	clientDir := filepath.Join(tmp, "client")
	var out strings.Builder
	code := run(context.Background(), []string{
		"clients", "go",
		"--openapi", specPath,
		"--out", clientDir,
		"--package", "apiclient",
		"--style", "typed",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("clients go --style typed failed: %s", out.String())
	}
	generated, err := os.ReadFile(filepath.Join(clientDir, "client.go"))
	if err != nil {
		t.Fatalf("read generated client.go: %v", err)
	}
	for _, want := range []string{
		"type Widget struct",
		"ID         string   `json:\"id\"`",
		"Version    int64    `json:\"version\"`",
		"NextCursor *string  `json:\"next_cursor,omitempty\"`",
		"Tags       []string `json:\"tags,omitempty\"`",
		"type WidgetCreateRequest struct",
		"func (c *Client) CreateWidget(ctx context.Context, body WidgetCreateRequest, opts ...RequestOption) (*Widget, *http.Response, error)",
		"func (c *Client) CreateWidgetRaw(ctx context.Context, body any, opts ...RequestOption) (*http.Response, error)",
		"type GetWidgetParams struct",
		"XTenantID      string",
		"IncludeDeleted *bool",
		"Header(\"X-Tenant-ID\", formatParamValue(params.XTenantID))",
		"QueryParam(\"include_deleted\", formatParamValue(*params.IncludeDeleted))",
		"func (c *Client) GetWidget(ctx context.Context, id string, params GetWidgetParams, opts ...RequestOption) (*Widget, *http.Response, error)",
		"func (c *Client) GetWidgetRaw(ctx context.Context, id string, params GetWidgetParams, opts ...RequestOption) (*http.Response, error)",
		"func DecodeJSONResponse[T any]",
		"func QueryParam",
		"func Header",
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated typed client.go missing %q:\n%s", want, generated)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/clienttest\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatalf("write temp go.mod: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = tmp
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated typed client package should build:\n%s\nerror: %v", output, err)
	}
}

func TestClientsTypeScriptGeneratesFetchPackage(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	spec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"},
				"BearerAuth": {"type": "http", "scheme": "bearer"}
			},
			"schemas": {
				"Problem": {
					"type": "object",
					"properties": {
						"type": {"type": "string"},
						"title": {"type": "string"},
						"status": {"type": "integer"},
						"detail": {"type": "string"}
					}
				},
				"Widget": {
					"type": "object",
					"required": ["id", "name", "state"],
					"properties": {
						"id": {"type": "string", "examples": ["wdg_123"]},
						"name": {"type": "string"},
						"state": {"type": "string", "enum": ["active", "archived"]},
						"next_cursor": {"type": ["string", "null"]},
						"tags": {"type": "array", "items": {"type": "string"}}
					}
				},
				"WidgetCreateRequest": {
					"type": "object",
					"required": ["name"],
					"properties": {
						"name": {"type": "string"}
					}
				}
			}
		},
		"paths": {
			"/widgets": {
				"post": {
					"operationId": "createWidget",
					"requestBody": {
						"required": true,
						"content": {"application/json": {"schema": {"$ref": "#/components/schemas/WidgetCreateRequest"}}}
					},
					"responses": {
						"201": {"description": "created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["widgets:write"]}]
				}
			},
			"/widgets/{id}": {
				"get": {
					"operationId": "getWidget",
					"parameters": [
						{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
						{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string"}},
						{"name": "include_deleted", "in": "query", "required": false, "schema": {"type": "boolean"}}
					],
					"responses": {
						"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}},
						"404": {"description": "not found", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"BearerAuth": ["widgets:read"]}]
				}
			}
		}
	}`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	clientDir := filepath.Join(tmp, "ts-client")
	var out strings.Builder
	code := run(context.Background(), []string{
		"clients", "typescript",
		"--openapi", specPath,
		"--out", clientDir,
		"--package-name", "@example/my-api-client",
		"--style", "fetch",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("clients typescript failed: %s", out.String())
	}
	index, err := os.ReadFile(filepath.Join(clientDir, "src", "index.ts"))
	if err != nil {
		t.Fatalf("read generated src/index.ts: %v", err)
	}
	for _, want := range []string{
		"export interface Widget",
		"id: string;",
		"state: \"active\" | \"archived\";",
		"next_cursor?: string | null;",
		"export interface WidgetCreateRequest",
		"export class ProblemDetailsError extends Error",
		"setAPIKey",
		"setBearerToken",
		"async createWidget(body: WidgetCreateRequest",
		"Promise<ClientResponse<Widget>>",
		"async getWidget(id: string, params: GetWidgetParams",
		"headers.set(\"X-Tenant-ID\", String(params.xTenantID));",
		"query.set(\"include_deleted\", String(params.includeDeleted));",
	} {
		if !strings.Contains(string(index), want) {
			t.Fatalf("generated src/index.ts missing %q:\n%s", want, index)
		}
	}
	pkg, err := os.ReadFile(filepath.Join(clientDir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(pkg), `"name": "@example/my-api-client"`) {
		t.Fatalf("package.json missing package name:\n%s", pkg)
	}
	for _, file := range []string{"tsconfig.json", "README.md"} {
		if _, err := os.Stat(filepath.Join(clientDir, file)); err != nil {
			t.Fatalf("expected %s: %v", file, err)
		}
	}
}

func TestClientsGoRejectsUnsafeInputs(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{}`)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "traversal output",
			args: []string{"clients", "go", "--openapi", specPath, "--out", "../escape", "--package", "apiclient"},
			want: "output directory must stay under the current working directory",
		},
		{
			name: "invalid package",
			args: []string{"clients", "go", "--openapi", specPath, "--out", filepath.Join(tmp, "client"), "--package", "api-client"},
			want: "invalid Go package name",
		},
		{
			name: "invalid style",
			args: []string{"clients", "go", "--openapi", specPath, "--out", filepath.Join(tmp, "client"), "--package", "apiclient", "--style", "custom"},
			want: "unsupported Go client style",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut strings.Builder
			code := run(context.Background(), tt.args, &strings.Builder{}, &errOut)
			if code == 0 {
				t.Fatal("expected clients go to fail")
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tt.want)
			}
		})
	}
}

func TestOpsObservabilityGeneratesBundle(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "observability")
	var out strings.Builder
	code := run(context.Background(), []string{
		"ops", "observability",
		"--profile", "saas-api-full",
		"--out", outDir,
	}, &out, &out)
	if code != 0 {
		t.Fatalf("ops observability failed: %s", out.String())
	}
	tests := map[string][]string{
		"grafana/saas-api-full-dashboard.json": {
			`"title": "api-toolkit saas-api-full"`,
			`http_server_duration_seconds`,
			`api_toolkit_outbox_jobs_total`,
		},
		"prometheus/saas-api-full-rules.yaml": {
			"ApiHighErrorRate",
			"ApiReadinessFailing",
			"WebhookDeliveryDeadLetters",
		},
		"runbooks/observability.md": {
			"tenant IDs are intentionally not metric labels",
			"admin endpoint isolation",
		},
	}
	for file, wants := range tests {
		data, err := os.ReadFile(filepath.Join(outDir, file))
		if err != nil {
			t.Fatalf("read generated %s: %v", file, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s missing %q:\n%s", file, want, data)
			}
		}
	}
}

func TestDeployGeneratorsGenerateHelmAndTerraformAWS(t *testing.T) {
	tmp := t.TempDir()
	helmDir := filepath.Join(tmp, "helm")
	var out strings.Builder
	code := run(context.Background(), []string{
		"deploy", "helm",
		"--dir", ".",
		"--out", helmDir,
	}, &out, &out)
	if code != 0 {
		t.Fatalf("deploy helm failed: %s", out.String())
	}
	for _, file := range []string{
		"Chart.yaml",
		"values.yaml",
		"templates/api-deployment.yaml",
		"templates/worker-deployment.yaml",
		"templates/migration-job.yaml",
		"templates/admin-service.yaml",
		"templates/network-policy.yaml",
		"templates/hpa.yaml",
		"templates/pdb.yaml",
	} {
		if _, err := os.Stat(filepath.Join(helmDir, file)); err != nil {
			t.Fatalf("expected helm file %s: %v", file, err)
		}
	}
	values, err := os.ReadFile(filepath.Join(helmDir, "values.yaml"))
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	for _, want := range []string{"livez", "readyz", "adminService:", "migration:"} {
		if !strings.Contains(string(values), want) {
			t.Fatalf("values.yaml missing %q:\n%s", want, values)
		}
	}

	terraformDir := filepath.Join(tmp, "terraform", "aws")
	out.Reset()
	code = run(context.Background(), []string{
		"deploy", "terraform",
		"--cloud", "aws",
		"--dir", ".",
		"--out", terraformDir,
	}, &out, &out)
	if code != 0 {
		t.Fatalf("deploy terraform failed: %s", out.String())
	}
	for _, file := range []string{"main.tf", "variables.tf", "outputs.tf", "README.md"} {
		if _, err := os.Stat(filepath.Join(terraformDir, file)); err != nil {
			t.Fatalf("expected terraform file %s: %v", file, err)
		}
	}
	mainTF, err := os.ReadFile(filepath.Join(terraformDir, "main.tf"))
	if err != nil {
		t.Fatalf("read main.tf: %v", err)
	}
	for _, want := range []string{"aws_db_instance", "aws_elasticache_replication_group", "aws_s3_bucket", "aws_iam_policy"} {
		if !strings.Contains(string(mainTF), want) {
			t.Fatalf("main.tf missing %q:\n%s", want, mainTF)
		}
	}
}

func TestNewServiceRejectsTraversalOutput(t *testing.T) {
	var errOut strings.Builder
	code := run(context.Background(), []string{"new", "service", "--module", "example.com/my-api", "--dir", "../escape"}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatalf("expected traversal output to fail")
	}
	if !strings.Contains(errOut.String(), "output directory must stay under the current working directory") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestNewServiceRejectsUnsupportedAuthModes(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		auth    string
		want    string
	}{
		{name: "dev headers require development profile", profile: "saas-api", auth: "dev-headers", want: "auth mode \"dev-headers\" requires an explicit development profile"},
		{name: "production modes stay on saas profile", profile: "dev-api", auth: "api-key", want: "auth mode \"api-key\" is not supported for profile \"dev-api\""},
		{name: "session requires web profile", profile: "saas-api", auth: "session", want: "auth mode \"session\" requires profile \"saas-web\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut strings.Builder
			code := run(context.Background(), []string{
				"new", "service",
				"--module", "example.com/my-api",
				"--profile", tt.profile,
				"--auth", tt.auth,
				"--dir", filepath.Join(t.TempDir(), "service"),
			}, &strings.Builder{}, &errOut)
			if code == 0 {
				t.Fatal("expected unsupported auth mode to fail")
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tt.want)
			}
		})
	}
}

func TestNewServiceRejectsUnsupportedProviderWorkflows(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		with    string
		want    string
	}{
		{name: "provider workflows require full profile", profile: "saas-api", with: "stripe-billing", want: "provider workflows require profile \"saas-api-full\""},
		{name: "unknown provider workflow", profile: "saas-api-full", with: "mailchimp", want: "unsupported provider workflow \"mailchimp\""},
		{name: "empty provider workflow", profile: "saas-api-full", with: " ", want: "provider workflow must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut strings.Builder
			code := run(context.Background(), []string{
				"new", "service",
				"--module", "example.com/my-api",
				"--profile", tt.profile,
				"--auth", "api-key",
				"--with", tt.with,
				"--dir", filepath.Join(t.TempDir(), "service"),
			}, &strings.Builder{}, &errOut)
			if code == 0 {
				t.Fatal("expected unsupported provider workflow to fail")
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", errOut.String(), tt.want)
			}
		})
	}
}

func TestNewServiceGeneratesFullProfileTypeScriptClientAndEntitlements(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--client", "typescript",
		"--with", "entitlements",
		"--dir", serviceDir,
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}
	for _, file := range []string{
		"internal/client/apiclient/client.go",
		"internal/client/ts/src/index.ts",
		"internal/entitlements/entitlements.go",
		"internal/entitlements/entitlements_test.go",
		"docs/providers/entitlements.md",
		"observability/prometheus/saas-api-full-rules.yaml",
		"deploy/helm/Chart.yaml",
		"deploy/terraform/aws/main.tf",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, file)); err != nil {
			t.Fatalf("expected generated file %s: %v", file, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(serviceDir, "api-toolkit.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	for _, want := range []string{
		"profile: saas-api-full",
		"typescript:",
		"path: internal/client/ts",
		"- entitlements",
	} {
		if !strings.Contains(string(manifest), want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
	makefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	for _, want := range []string{"client-ts-check", "provider-check", "migrate-plan", "migrate-verify", "migrate-down"} {
		if !strings.Contains(string(makefile), want) {
			t.Fatalf("Makefile missing %q:\n%s", want, makefile)
		}
	}
	migrateCmd, err := os.ReadFile(filepath.Join(serviceDir, "cmd/migrate/main.go"))
	if err != nil {
		t.Fatalf("read cmd/migrate/main.go: %v", err)
	}
	for _, want := range []string{
		"bootstrap.RunDown(ctx, migrator, *dir)",
		"ALLOW_DANGEROUS_MIGRATION_DOWN",
		"one migration reverted",
	} {
		if !strings.Contains(string(migrateCmd), want) {
			t.Fatalf("migrate command missing %q:\n%s", want, migrateCmd)
		}
	}
}

func TestNewServiceGeneratesSaaSWebSessionProfile(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "web")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-web",
		"--profile", "saas-web",
		"--auth", "session",
		"--dir", serviceDir,
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new saas-web service failed: %s", out.String())
	}
	for _, file := range []string{
		"go.mod",
		"cmd/web/main.go",
		"internal/session/session.go",
		"internal/session/session_test.go",
		"README.md",
		"Makefile",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, file)); err != nil {
			t.Fatalf("expected saas-web file %s: %v", file, err)
		}
	}
	sessionCode, err := os.ReadFile(filepath.Join(serviceDir, "internal/session/session.go"))
	if err != nil {
		t.Fatalf("read session.go: %v", err)
	}
	for _, want := range []string{
		"SameSite=Lax",
		"HttpOnly",
		"Secure",
		"Rotate",
		"ValidateCSRF",
		"NewRedisStore",
		"CSRFMiddleware",
		"BeginOIDCLogin",
		"ValidateOIDCCallback",
		"BrowserSafeCORS",
		"SESSION_SECRET must be at least 32 characters in production",
	} {
		if !strings.Contains(string(sessionCode), want) {
			t.Fatalf("session.go missing %q:\n%s", want, sessionCode)
		}
	}
	sessionTests, err := os.ReadFile(filepath.Join(serviceDir, "internal/session/session_test.go"))
	if err != nil {
		t.Fatalf("read session_test.go: %v", err)
	}
	for _, want := range []string{
		"TestProductionValidationRequiresSecret",
		"TestCSRFMiddlewareRejectsMissingToken",
		"TestSessionRotationPreventsFixation",
		"TestOIDCCallbackValidatesState",
		"TestBrowserSafeCORSRejectsWildcardAndUnknownOrigin",
	} {
		if !strings.Contains(string(sessionTests), want) {
			t.Fatalf("session tests missing %q:\n%s", want, sessionTests)
		}
	}
	goMod, err := os.ReadFile(filepath.Join(serviceDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "github.com/redis/go-redis/v9") {
		t.Fatalf("go.mod missing Redis session dependency:\n%s", goMod)
	}
	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated saas-web tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated saas-web tests failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableDevAPIWithDevHeaders(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "dev-api",
		"--auth", "dev-headers",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("DevHeaderAuth"`,
		"newDevHeadersMiddleware",
		"withDevHeaderAuthorizationScope",
		"requireDevHeaderScope",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated dev-header main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{
		"DEV_AUTH_FALLBACK_ENABLED=true",
		"DEV_AUTH_ALLOW_DANGEROUS_DEV_BYPASSES=true",
		"DEV_AUTH_TENANT_HEADER=X-Debug-Tenant-ID",
		"DEV_AUTH_SCOPE_HEADER=X-Debug-Scopes",
	} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated dev-header .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `dev-headers`.") {
		t.Fatalf("generated README missing dev-header auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "DevHeaderAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dev-header service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIWithClerk(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--auth", "clerk",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("BearerAuth"`,
		"newClerkMiddleware",
		"ShutdownHooks:",
		"withClerkAuthorizationScope",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated Clerk main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"CLERK_JWKS_URL=", "CLERK_ISSUER=", "CLERK_AUDIENCE=saas-api"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated Clerk .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `clerk`.") {
		t.Fatalf("generated README missing Clerk auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Clerk service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIWithJWT(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--auth", "jwt",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`RegisterSecurityScheme("BearerAuth"`,
		"newJWTMiddleware",
		"ShutdownHooks:",
		"tenantIDFromJWTSubject",
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated JWT main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"JWT_JWKS_URL=", "JWT_ISSUER=", "JWT_AUDIENCE=saas-api"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated JWT .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `jwt`.") {
		t.Fatalf("generated README missing JWT auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT service contracts lint failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPI(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new service failed: %s", out.String())
	}
	for _, name := range []string{"go.mod", "main.go", "main_test.go", "testdata/openapi.golden.json", "Makefile", ".env.example", ".gitignore", ".dockerignore", ".github/workflows/ci.yml", "Dockerfile", "docker-compose.yml", "README.md"} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "main.go"))
	if err != nil {
		t.Fatalf("read generated main.go: %v", err)
	}
	for _, want := range []string{
		`appVersion  = "dev"`,
		`buildCommit = "unknown"`,
		`buildDate   = "unknown"`,
		"MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder()",
		"NewPrometheusRecorderChecked",
		"bootstrap.DefaultRouterConfigFromEnv(nil)",
		"routerConfig.Metrics = metricsRecorder",
		"HealthStatusChangeHook",
		"IdempotencyOutcomeHook",
		"IdempotencyOutcomeLogHook",
		"BackgroundTasks:",
		"newTracingShutdown(",
		"telemetry.InitTracing",
		`bootstrap.ShutdownHook{Name: "otel-tracing"`,
		"newRateLimitLimiter(",
		"ratelimitredis.New",
		`bootstrap.ShutdownHook{Name: "rate-limit-redis"`,
		"newIdempotencyStore()",
		`bootstrap.ShutdownHook{Name: "idempotency-redis"`,
		"client.Close()",
		"StorageKeyFunc: idempotencymw.TenantScopedStorageKeyFunc()",
		"idempotencyredis.New",
		`ports.VersionInfo{Version: appVersion, Commit: buildCommit, Date: buildDate}`,
	} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated main.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated .env.example: %v", err)
	}
	for _, want := range []string{"IDEMPOTENCY_STORE=memory", "REDIS_ADDR=localhost:6379"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated .env.example missing %q", want)
		}
	}
	assertGeneratedEnvDocumentsRateLimitConfig(t, string(generatedEnv))
	assertGeneratedEnvDocumentsTelemetryConfig(t, string(generatedEnv))
	generatedDockerfile, err := os.ReadFile(filepath.Join(serviceDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read generated Dockerfile: %v", err)
	}
	for _, want := range []string{
		"FROM golang:1.25 AS build",
		"ARG VERSION=dev",
		"ARG BUILD_COMMIT=unknown",
		"ARG BUILD_DATE=unknown",
		"CGO_ENABLED=0 GOOS=linux go build",
		"-X main.appVersion=${VERSION}",
		"-X main.buildCommit=${BUILD_COMMIT}",
		"-X main.buildDate=${BUILD_DATE}",
		"USER nonroot:nonroot",
		"ENTRYPOINT [\"/api\"]",
	} {
		if !strings.Contains(string(generatedDockerfile), want) {
			t.Fatalf("generated Dockerfile missing %q", want)
		}
	}
	if strings.Contains(string(generatedDockerfile), "go run") {
		t.Fatalf("generated Dockerfile should run a built binary, got:\n%s", generatedDockerfile)
	}
	generatedDockerignore, err := os.ReadFile(filepath.Join(serviceDir, ".dockerignore"))
	if err != nil {
		t.Fatalf("read generated .dockerignore: %v", err)
	}
	for _, want := range []string{".env", ".git", ".tools"} {
		if !strings.Contains(string(generatedDockerignore), want) {
			t.Fatalf("generated .dockerignore missing %q", want)
		}
	}
	generatedGitignore, err := os.ReadFile(filepath.Join(serviceDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read generated .gitignore: %v", err)
	}
	for _, want := range []string{".env", "!.env.example", "coverage.out", ".ci-result/", ".tools/"} {
		if !strings.Contains(string(generatedGitignore), want) {
			t.Fatalf("generated .gitignore missing %q", want)
		}
	}
	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}
	for _, want := range []string{
		"VERSION ?= dev",
		"BUILD_COMMIT ?=",
		"BUILD_DATE ?=",
		"LDFLAGS ?=",
		"OUTPUT_DIR ?=",
		"TOOLS_DIR ?=",
		"SYFT ?=",
		"COVERAGE_MIN ?=",
		"GOVULNCHECK ?=",
		"GOVULNCHECK_VERSION ?=",
		"tools:",
		"GOBIN=\"$(CURDIR)/$(TOOLS_DIR)/bin\" $(GO) install",
		"build:",
		"-X main.appVersion=$(VERSION)",
		"coverage-check:",
		"tool cover -func=coverage.out",
		"test-race:",
		"vuln:",
		"\"$(GOVULNCHECK)\" ./...",
		"contracts-lint:",
		"contracts-diff:",
		"fast-check:",
		"audit-check:",
		"sbom-local:",
		"clean:",
		"finalize: fmt audit-check clean",
		"API_TOOLKIT ?=",
		"OPENAPI_BASE ?=",
	} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated Makefile missing %q", want)
		}
	}
	generatedCI, err := os.ReadFile(filepath.Join(serviceDir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read generated CI workflow: %v", err)
	}
	for _, want := range []string{
		"name: ci",
		"pull_request:",
		"permissions:",
		"contents: read",
		"GOTOOLCHAIN: local",
		"actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5",
		"actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff",
		"go-version: 1.25.x",
		"make finalize",
	} {
		if !strings.Contains(string(generatedCI), want) {
			t.Fatalf("generated CI workflow missing %q:\n%s", want, generatedCI)
		}
	}
	generatedCompose, err := os.ReadFile(filepath.Join(serviceDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read generated docker-compose.yml: %v", err)
	}
	for _, want := range []string{
		"redis:",
		"image: redis:7-alpine",
		"REDIS_ADDR: redis:6379",
		"RATE_LIMIT_REDIS_ADDR: redis:6379",
		"redis-data:",
		`test: ["CMD", "redis-cli", "ping"]`,
	} {
		if !strings.Contains(string(generatedCompose), want) {
			t.Fatalf("generated docker-compose.yml missing %q:\n%s", want, generatedCompose)
		}
	}
	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README.md: %v", err)
	}
	if !strings.Contains(string(generatedREADME), "Generated auth mode: `api-key`.") {
		t.Fatalf("generated README missing auth mode")
	}
	assertGeneratedREADMEListsOperatorRoutes(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsIdempotencyRequirement(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsRateLimitStore(t, string(generatedREADME))
	assertGeneratedREADMEDocumentsTelemetry(t, string(generatedREADME))
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "ApiKeyAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "openapi-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service openapi check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service contracts lint failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-diff")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service contracts diff failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "build")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service build failed:\n%s\nerror: %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(serviceDir, "bin", "api")); err != nil {
		t.Fatalf("expected generated build artifact: %v", err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "coverage-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service coverage check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "coverage-check")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "COVERAGE_MIN=100.0")
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("generated service coverage check should fail below an explicit 100%% floor:\n%s", output)
	}
	if !strings.Contains(string(output), "coverage") || !strings.Contains(string(output), "below required") {
		t.Fatalf("generated service coverage failure should explain the floor:\n%s", output)
	}
	cmd = exec.CommandContext(context.Background(), "make", "vuln", "GOVULNCHECK=/bin/true")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service vuln target should use overridable local tool path:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "test-race")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated service race tests failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFull(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service failed: %s", out.String())
	}

	for _, name := range []string{
		"go.mod",
		"cmd/api/main.go",
		"cmd/worker/main.go",
		"internal/domain/api_key.go",
		"internal/domain/tenancy.go",
		"internal/domain/widget.go",
		"internal/app/audit.go",
		"internal/app/audit_test.go",
		"internal/app/cache.go",
		"internal/app/cache_test.go",
		"internal/app/api_keys.go",
		"internal/app/api_keys_test.go",
		"internal/app/async.go",
		"internal/app/async_test.go",
		"internal/app/tenancy.go",
		"internal/app/tenancy_test.go",
		"internal/app/widgets.go",
		"internal/app/webhooks.go",
		"internal/app/webhooks_test.go",
		"internal/app/objects.go",
		"internal/app/objects_test.go",
		"internal/adapters/postgres/postgres.go",
		"internal/adapters/postgres/postgres_test.go",
		"internal/adapters/postgres/widgets.go",
		"internal/adapters/postgres/widgets_test.go",
		"internal/adapters/postgres/objects.go",
		"internal/adapters/postgres/objects_test.go",
		"internal/adapters/postgres/webhooks.go",
		"internal/adapters/postgres/webhooks_test.go",
		"internal/adapters/redis/cache.go",
		"internal/adapters/redis/cache_test.go",
		"internal/httpapi/router.go",
		"internal/httpapi/router_test.go",
		"internal/httpapi/openapi.go",
		"internal/client/apiclient/client.go",
		"api-toolkit.yaml",
		"cmd/migrate/main.go",
		"migrations/20260517000100_platform.up.sql",
		"migrations/20260517000100_platform.down.sql",
		"scripts/integration_check.sh",
		"testdata/openapi.golden.json",
		"Makefile",
		".env.example",
		".gitignore",
		".dockerignore",
		".github/workflows/ci.yml",
		".github/workflows/integration.yml",
		"Dockerfile",
		"docker-compose.yml",
		"deploy/kubernetes/configmap.yaml",
		"deploy/kubernetes/secret.example.yaml",
		"deploy/kubernetes/migration-job.yaml",
		"deploy/kubernetes/deployment.yaml",
		"deploy/kubernetes/worker-deployment.yaml",
		"deploy/kubernetes/service.yaml",
		"deploy/kubernetes/admin-service.yaml",
		"deploy/kubernetes/pod-disruption-budget.yaml",
		"deploy/kubernetes/hpa.yaml",
		"deploy/kubernetes/network-policy.yaml",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated full-profile %s: %v", name, err)
		}
	}

	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated full-profile .env.example: %v", err)
	}

	generatedManifest, err := os.ReadFile(filepath.Join(serviceDir, "api-toolkit.yaml"))
	if err != nil {
		t.Fatalf("read generated api-toolkit.yaml: %v", err)
	}
	for _, want := range []string{"profile: saas-api-full", "module: example.com/my-api", "openapi: testdata/openapi.golden.json", "generator_version:", "resources:", "providers:"} {
		if !strings.Contains(string(generatedManifest), want) {
			t.Fatalf("generated api-toolkit.yaml missing %q:\n%s", want, generatedManifest)
		}
	}
	for _, want := range []string{
		"DATABASE_URL=",
		"REDIS_ADDR=localhost:6379",
		"CACHE_STORE=memory",
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_KEY_PREFIX=ratelimit:",
		"IDEMPOTENCY_STORE=memory",
		"IDEMPOTENCY_KEY_PREFIX=idempotency:",
		"OPENAPI_REQUEST_VALIDATION=true",
		"OPENAPI_RESPONSE_VALIDATION=true",
		"ASYNC_WORKER_ENABLED=true",
		"API_ACTOR_ID=",
		"API_KEY_PEPPER=",
		"WEBHOOK_SECRET_KEY=",
		"ADMIN_ADDR=:9090",
	} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated full-profile .env.example missing %q", want)
		}
	}

	generatedMigration, err := os.ReadFile(filepath.Join(serviceDir, "migrations", "20260517000100_platform.up.sql"))
	if err != nil {
		t.Fatalf("read generated migration: %v", err)
	}
	for _, want := range []string{
		"CREATE TABLE organizations",
		"CREATE TABLE memberships",
		"CREATE TABLE invitations",
		"CREATE TABLE api_keys",
		"CREATE TABLE widgets",
		"CREATE TABLE operations",
		"CREATE TABLE outbox_events",
		"CREATE TABLE audit_events",
		"actor_type TEXT NOT NULL",
		"CREATE TABLE objects",
		"content_type TEXT NOT NULL",
		"CREATE TABLE webhook_endpoints",
		"secret_ciphertext BYTEA NOT NULL",
		"secret_nonce BYTEA NOT NULL",
		"CREATE TABLE webhook_deliveries",
		"event_id TEXT NOT NULL",
		"last_status_code INTEGER",
	} {
		if !strings.Contains(string(generatedMigration), want) {
			t.Fatalf("generated migration missing %q", want)
		}
	}

	generatedMigrate, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "migrate", "main.go"))
	if err != nil {
		t.Fatalf("read generated migrate main.go: %v", err)
	}
	for _, want := range []string{"command required: plan | up | status | check | verify | down", "DATABASE_URL is required", "MIGRATIONS_DIR", "migrations", "bootstrap.NewMigrator", "bootstrap.RunUp", "bootstrap.Status", `strings.Contains(status, "*")`, "ALLOW_DANGEROUS_MIGRATION_DOWN"} {
		if !strings.Contains(string(generatedMigrate), want) {
			t.Fatalf("generated migrate main.go missing %q", want)
		}
	}

	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full-profile router: %v", err)
	}
	for _, want := range []string{"If-Match", "ETag", "Idempotency-Key", "X-Tenant-ID", "WriteProblem", "corepprof.RegisterAdminRoutes", "apiKeyPrincipalFromContext", "required API key scope missing", "NewRateLimitMiddleware", "NewMetricsMiddleware", "RegisterRoutes", "NewHealthHandler", "NewOpenAPIValidationMiddleware", "OpenAPIValidation", "/livez", "rateLimited", "metrics", "MetricsHandler", "handleCreateOrganization", "handleCreateInvitation", "handleCreateAPIKey", "handleRevokeAPIKey", "handleCreateWidgetImport", "handleGetOperation", "handleCreateWebhookEndpoint", "handleReplayWebhookDelivery", "handlePutObject", "handleGetObject", "recordAudit", "Readiness"} {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full-profile router missing %q", want)
		}
	}
	generatedRouterTest, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router_test.go"))
	if err != nil {
		t.Fatalf("read generated full-profile router_test.go: %v", err)
	}
	for _, want := range []string{"configureTestAuthEnv(t)", `t.Setenv("API_ACTOR_ID", "")`} {
		if !strings.Contains(string(generatedRouterTest), want) {
			t.Fatalf("generated full-profile router_test.go missing %q", want)
		}
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full-profile main.go: %v", err)
	}
	for _, want := range []string{"postgres.Open", "postgres.CheckRequiredTables", "postgres.NewWidgetStore", "app.NewWidgetServiceWithStore", "postgres.NewTenancyStore", "app.NewTenancyServiceWithStore", "postgres.NewAPIKeyStore", "app.NewAPIKeyServiceWithStore", "postgres.NewWidgetImportOperationStore", "postgres.NewWidgetImportOutbox", "app.NewAsyncServiceWithStores", "postgres.NewWebhookStore", "postgres.NewWebhookStore(postgresPool, cfg.WebhookSecretKey, webhookEndpointPolicy)", "app.NewWebhookServiceWithStoreAndEndpointPolicy", "cfg.WebhookSecretKey", "postgres.NewObjectStore", "app.NewObjectServiceWithStores", "objectstorage.OpenS3BlobStore", "auditpostgres.New", "pgxpooladapter.Adapter", "app.NewAuditServiceWithRecorder", "webhookdelivery.EndpointPolicy", "webhookdelivery.NewDeliverer", "webhookdelivery.NewHandler", "webhookdeliverypostgres.OutboxEventType", "async.NewHandlerMux", "Handler:      asyncHandler", "backgroundTasks", "cfg.AsyncWorkerEnabled", "rediscache.OpenCache", "rediscache.OpenRateLimiter", "rediscache.OpenIdempotencyStore", "metricsmw.NewPrometheusRecorderChecked", "httpapi.NewMetricsMiddleware", "httpapi.NewRateLimitMiddleware", "httpapi.NewIdempotencyMiddleware", "httpapi.NewOpenAPIValidationMiddleware", "bootstrap.NewAPIService", "httpapi.RegisterRoutes", "bootstrap.StrictSaaSAPIMiddlewareOrder", "AdminAddr:               cfg.AdminAddr", "httpapi.NewHealthHandler", "version.NewHandler", "metricsmw.PrometheusHandler", "app.NewAuditService", "app.NewWebhookServiceWithEndpointPolicy", "app.NewObjectService", "app.NewCacheService", "Audit:", "Webhooks:", "Objects:", "Cache:", "Metrics:", "MetricsHandler:", "OpenAPIValidation:", "RateLimit:", "Idempotency:", "Readiness:"} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full-profile main.go missing %q", want)
		}
	}

	generatedWorker, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "worker", "main.go"))
	if err != nil {
		t.Fatalf("read generated worker main.go: %v", err)
	}
	for _, want := range []string{"DATABASE_URL is required for worker", "postgres.NewWidgetImportOutbox", "webhookdelivery.NewHandler", "async.NewHandlerMux", "asyncRunner.Run(ctx)"} {
		if !strings.Contains(string(generatedWorker), want) {
			t.Fatalf("generated worker main.go missing %q", want)
		}
	}

	generatedWebhooks, err := os.ReadFile(filepath.Join(serviceDir, "internal", "app", "webhooks.go"))
	if err != nil {
		t.Fatalf("read generated app webhooks: %v", err)
	}
	for _, want := range []string{"NewWebhookServiceWithEndpointPolicy", "EndpointPolicy: s.endpointPolicy", "RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error", "webhookdelivery.AttemptRecorder"} {
		if !strings.Contains(string(generatedWebhooks), want) {
			t.Fatalf("generated app webhooks missing %q", want)
		}
	}

	generatedPostgresWebhooks, err := os.ReadFile(filepath.Join(serviceDir, "internal", "adapters", "postgres", "webhooks.go"))
	if err != nil {
		t.Fatalf("read generated postgres webhooks: %v", err)
	}
	for _, want := range []string{"endpointPolicy webhookdelivery.EndpointPolicy", "NewWebhookStore(pool ports.DatabasePool, secretKey string, endpointPolicy webhookdelivery.EndpointPolicy)", "EndpointPolicy: endpointPolicy", "webhookdelivery.ValidateEndpoint(endpoint, s.endpointPolicy)", "RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error", "s.base.RecordAttempt(ctx, result)"} {
		if !strings.Contains(string(generatedPostgresWebhooks), want) {
			t.Fatalf("generated postgres webhooks missing %q", want)
		}
	}

	generatedClient, err := os.ReadFile(filepath.Join(serviceDir, "internal", "client", "apiclient", "client.go"))
	if err != nil {
		t.Fatalf("read generated full-profile client: %v", err)
	}
	for _, want := range []string{"package apiclient", "func (c *Client) CreateWidget", "func (c *Client) CreateWidgetImport", "func (c *Client) GetOperation", "func (c *Client) UpdateWidget", "func (c *Client) CreateOrganization", "func (c *Client) CreateOrganizationInvitation", "func (c *Client) CreateOrganizationAPIKey", "func (c *Client) RevokeOrganizationAPIKey", "func (c *Client) CreateOrganizationWebhookEndpoint", "func (c *Client) ReplayOrganizationWebhookDelivery", "func (c *Client) PutOrganizationObject", "func (c *Client) GetOrganizationObject", "func (c *Client) DeleteOrganizationObject", "PathParam(\"id\",", "PathParam(\"organization_id\",", "PathParam(\"api_key_id\",", "PathParam(\"delivery_id\",", "PathParam(\"object_key\","} {
		if !strings.Contains(string(generatedClient), want) {
			t.Fatalf("generated full-profile client missing %q", want)
		}
	}
	for _, want := range []string{
		"type Widget struct",
		"type WidgetCreateRequest struct",
		"type ListWidgetsParams struct",
		"XTenantID string",
		"Cursor    *string",
		"Limit     *int",
		"func (c *Client) ListWidgets(ctx context.Context, params ListWidgetsParams, opts ...RequestOption) (*WidgetList, *http.Response, error)",
		"func (c *Client) CreateWidget(ctx context.Context, params CreateWidgetParams, body WidgetCreateRequest, opts ...RequestOption) (*Widget, *http.Response, error)",
		"func (c *Client) UpdateWidget(ctx context.Context, id string, params UpdateWidgetParams, body WidgetCreateRequest, opts ...RequestOption) (*Widget, *http.Response, error)",
		"Header(\"X-Tenant-ID\", formatParamValue(params.XTenantID))",
		"Header(\"Idempotency-Key\", formatParamValue(params.IdempotencyKey))",
	} {
		if !strings.Contains(string(generatedClient), want) {
			t.Fatalf("generated full-profile typed client missing %q", want)
		}
	}

	generatedMakefile, err := os.ReadFile(filepath.Join(serviceDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated full-profile Makefile: %v", err)
	}
	for _, want := range []string{"deps:", "mod tidy", "openapi-check:", "contracts-lint:", "contracts-diff:", "integration-check:", "client-check:", "resource-check:", "migrate-up:", "migrate-status:", "migrate-check:", "--style typed", "$(GO) build -trimpath -o bin/migrate ./cmd/migrate", "$(GO) build -trimpath -o bin/worker ./cmd/worker", "bash scripts/integration_check.sh"} {
		if !strings.Contains(string(generatedMakefile), want) {
			t.Fatalf("generated full-profile Makefile missing %q", want)
		}
	}
	generatedIntegration, err := os.ReadFile(filepath.Join(serviceDir, "scripts", "integration_check.sh"))
	if err != nil {
		t.Fatalf("read generated full-profile integration script: %v", err)
	}
	for _, want := range []string{
		"compose up -d postgres redis",
		"cp .env.example .env",
		"go run ./cmd/migrate up",
		"go run ./cmd/migrate check",
		"psql -v ON_ERROR_STOP=1 -U api -d api",
		"go mod tidy",
		"go test ./...",
		"go run ./cmd/api",
		"go run ./cmd/worker",
		"ASYNC_WORKER_ENABLED=false",
		"WEBHOOK_RECEIVER_ADDR",
		"webhook_receiver_log",
		"webhook delivery did not succeed",
		"failing webhook did not record retryable failure",
		"outbox dead-letter was not recorded",
		"command -v python3",
		"/livez",
		"/docs/openapi.json",
		"/health/detailed",
		"/metrics",
		"/debug/pprof/",
		"INTEGRATION_OBJECT_STORE",
		"wait_for_minio",
		"minio-init",
		"X-API-Key: ${API_KEY}",
		"X-Actor-ID: ${API_ACTOR_ID}",
		"Idempotency-Key:",
		"/organizations/${org_id}/api-keys",
		"/organizations/${org_id}/webhook-endpoints",
		"/organizations/${org_id}/webhook-deliveries",
		"managed-key-widget",
		"expected idempotent widget replay",
		"If-Match: stale-etag",
		"/widgets/imports",
		"operation did not succeed",
		"psql_exec \"insert into outbox_events",
		"operation outbox did not complete",
		"integration-poison-outbox",
		"psql_scalar \"select state from outbox_events",
		"outbox retry was not recorded",
		"webhook delivery list leaked signing secret",
		"replay webhook delivery",
		"object get did not return stored content",
		"audit_events",
		"audit events were not recorded",
		"public pprof endpoint should be isolated",
		"docker compose --profile objectstore down -v",
	} {
		if !strings.Contains(string(generatedIntegration), want) {
			t.Fatalf("generated full-profile integration script missing %q", want)
		}
	}
	cmd := exec.CommandContext(context.Background(), "bash", "-n", "scripts/integration_check.sh")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile integration script syntax failed:\n%s\nerror: %v", output, err)
	}

	generatedCompose, err := os.ReadFile(filepath.Join(serviceDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read generated full-profile compose: %v", err)
	}
	for _, want := range []string{
		"postgres:",
		"image: postgres:18-alpine",
		"migrate:",
		"entrypoint: [\"/migrate\"]",
		"command: [\"-dir\", \"/migrations\", \"up\"]",
		"service_completed_successfully",
		"worker:",
		"entrypoint: [\"/worker\"]",
		"/var/lib/postgresql",
		"redis:",
		"image: redis:7-alpine",
		"minio:",
		"profiles: [objectstore]",
		"minio-init:",
		"mc mb --ignore-existing local/api-objects",
	} {
		if !strings.Contains(string(generatedCompose), want) {
			t.Fatalf("generated full-profile docker-compose.yml missing %q:\n%s", want, generatedCompose)
		}
	}
	if strings.Contains(string(generatedCompose), "mc mb -p --ignore-existing") {
		t.Fatalf("generated full-profile docker-compose.yml uses incompatible mc flags:\n%s", generatedCompose)
	}

	generatedDockerfile, err := os.ReadFile(filepath.Join(serviceDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read generated full-profile Dockerfile: %v", err)
	}
	for _, want := range []string{"go mod tidy", "go test ./...", "go build -trimpath -o /out/api ./cmd/api", "go build -trimpath -o /out/worker ./cmd/worker", "go build -trimpath -o /out/migrate ./cmd/migrate", "COPY --from=build /out/worker /worker", "COPY --from=build /out/migrate /migrate", "COPY migrations /migrations"} {
		if !strings.Contains(string(generatedDockerfile), want) {
			t.Fatalf("generated full-profile Dockerfile missing %q:\n%s", want, generatedDockerfile)
		}
	}

	generatedDeployment, err := os.ReadFile(filepath.Join(serviceDir, "deploy", "kubernetes", "deployment.yaml"))
	if err != nil {
		t.Fatalf("read generated deployment: %v", err)
	}
	for _, want := range []string{"configMapRef", "secretKeyRef", "runAsNonRoot: true", "readOnlyRootFilesystem: true", "requests:", "limits:", "path: /livez", "path: /readyz", "ASYNC_WORKER_ENABLED"} {
		if !strings.Contains(string(generatedDeployment), want) {
			t.Fatalf("generated deployment missing %q:\n%s", want, generatedDeployment)
		}
	}

	generatedWorkerDeployment, err := os.ReadFile(filepath.Join(serviceDir, "deploy", "kubernetes", "worker-deployment.yaml"))
	if err != nil {
		t.Fatalf("read generated worker deployment: %v", err)
	}
	for _, want := range []string{"name: api-worker", "command: [\"/worker\"]", "configMapRef", "secretKeyRef", "runAsNonRoot: true", "readOnlyRootFilesystem: true", "requests:", "limits:", "DATABASE_URL", "WEBHOOK_SECRET_KEY"} {
		if !strings.Contains(string(generatedWorkerDeployment), want) {
			t.Fatalf("generated worker deployment missing %q:\n%s", want, generatedWorkerDeployment)
		}
	}

	for name, wants := range map[string][]string{
		"configmap.yaml":             {"kind: ConfigMap", "OPENAPI_REQUEST_VALIDATION", "ASYNC_WORKER_ENABLED"},
		"secret.example.yaml":        {"kind: Secret", "database-url", "api-key-pepper", "webhook-secret-key"},
		"migration-job.yaml":         {"kind: Job", "command: [\"/migrate\"]", "args: [\"-dir\", \"/migrations\", \"up\"]", "restartPolicy: OnFailure", "backoffLimit: 3"},
		"admin-service.yaml":         {"kind: Service", "api-toolkit.dev/internal-only: \"true\"", "type: ClusterIP"},
		"pod-disruption-budget.yaml": {"kind: PodDisruptionBudget", "minAvailable: 1"},
		"hpa.yaml":                   {"kind: HorizontalPodAutoscaler", "minReplicas: 2", "maxReplicas: 10"},
		"network-policy.yaml":        {"kind: NetworkPolicy", "policyTypes:", "Ingress", "Egress"},
	} {
		generatedAsset, err := os.ReadFile(filepath.Join(serviceDir, "deploy", "kubernetes", name))
		if err != nil {
			t.Fatalf("read generated kubernetes %s: %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(generatedAsset), want) {
				t.Fatalf("generated kubernetes %s missing %q:\n%s", name, want, generatedAsset)
			}
		}
	}

	generatedIntegrationWorkflow, err := os.ReadFile(filepath.Join(serviceDir, ".github", "workflows", "integration.yml"))
	if err != nil {
		t.Fatalf("read generated integration workflow: %v", err)
	}
	for _, want := range []string{"workflow_dispatch:", "schedule:", "make integration-check", "docker compose"} {
		if !strings.Contains(string(generatedIntegrationWorkflow), want) {
			t.Fatalf("generated integration workflow missing %q:\n%s", want, generatedIntegrationWorkflow)
		}
	}

	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated full-profile README.md: %v", err)
	}
	for _, want := range []string{
		"Generated profile: `saas-api-full`.",
		"Postgres stores tenants, API keys, widgets, operations, outbox, audit, webhook delivery state, and object metadata.",
		"The generated binary uses `bootstrap.NewAPIService`",
		"`cmd/worker` runs background jobs without serving public HTTP traffic.",
		"`cmd/migrate` applies and checks contrib migrator-compatible SQL files under `migrations/`.",
		"`/livez` is a process liveness probe",
		"Runtime OpenAPI request validation is enabled by default.",
		"The public router emits bounded Prometheus HTTP request metrics, and `/metrics` is served only from the admin router.",
		"The admin router mounts real Go pprof handlers behind `X-Admin-Key`; the public router does not mount pprof when `ADMIN_ADDR` is set.",
		"API-key mode keeps `API_KEY` as a bootstrap setup credential and verifies generated scoped API keys through the API-key service after setup.",
		"`make integration-check`",
	} {
		if !strings.Contains(string(generatedREADME), want) {
			t.Fatalf("generated full-profile README.md missing %q", want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "ApiKeyAuth")
	assertGeneratedGoldenOpenAPIVersion(t, serviceDir, "3.1.0")

	cmd = exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "openapi-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service openapi check failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-lint")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service contracts lint failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "contracts-diff")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service contracts diff failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "client-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full-profile service client check failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesFullProfileProviderWorkflows(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "api-key",
		"--with", "stripe-billing",
		"--with", "resend-email",
		"--with", "clerk-webhooks",
		"--with", "entitlements",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service with providers failed: %s", out.String())
	}

	for _, name := range []string{
		"internal/providers/stripebilling/billing.go",
		"internal/providers/stripebilling/billing_test.go",
		"internal/providers/resendemail/invitations.go",
		"internal/providers/resendemail/invitations_test.go",
		"internal/providers/clerkwebhooks/webhooks.go",
		"internal/providers/clerkwebhooks/webhooks_test.go",
		"internal/entitlements/entitlements.go",
		"internal/entitlements/entitlements_test.go",
		"cmd/provider-replay/main.go",
		"cmd/provider-replay/main_test.go",
		"testdata/providers/stripe-webhook.json",
		"testdata/providers/resend-invitation.json",
		"testdata/providers/clerk-webhook.json",
		"docs/providers/stripe-billing.md",
		"docs/providers/resend-email.md",
		"docs/providers/clerk-webhooks.md",
		"docs/providers/entitlements.md",
		"docs/providers/provider-runbook.md",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated provider workflow %s: %v", name, err)
		}
	}

	generatedManifest, err := os.ReadFile(filepath.Join(serviceDir, "api-toolkit.yaml"))
	if err != nil {
		t.Fatalf("read generated manifest: %v", err)
	}
	for _, want := range []string{"- stripe-billing", "- resend-email", "- clerk-webhooks", "- entitlements"} {
		if !strings.Contains(string(generatedManifest), want) {
			t.Fatalf("generated manifest missing %q:\n%s", want, generatedManifest)
		}
	}

	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated env: %v", err)
	}
	for _, want := range []string{
		"STRIPE_SECRET_KEY=",
		"STRIPE_WEBHOOK_SECRET=",
		"STRIPE_PRICE_ID=",
		"STRIPE_SUCCESS_URL=http://localhost:8080/billing/success",
		"STRIPE_CANCEL_URL=http://localhost:8080/billing/cancel",
		"RESEND_API_KEY=",
		"RESEND_FROM=",
		"APP_BASE_URL=http://localhost:8080",
		"CLERK_WEBHOOK_SECRET=",
	} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated env missing provider value %q:\n%s", want, generatedEnv)
		}
	}

	generatedREADME, err := os.ReadFile(filepath.Join(serviceDir, "README.md"))
	if err != nil {
		t.Fatalf("read generated README: %v", err)
	}
	for _, want := range []string{
		"Optional provider workflows generated: `stripe-billing`, `resend-email`, `clerk-webhooks`, `entitlements`.",
		"`internal/providers/stripebilling` creates tenant-scoped checkout sessions and verifies Stripe webhooks before audit writes.",
		"`internal/providers/resendemail` sends invitation emails through a sender boundary with a no-op local fallback.",
		"`internal/providers/clerkwebhooks` verifies signed Clerk callbacks before user or organization sync hooks run.",
		"`internal/entitlements` provides provider-neutral plan, feature, quota, and usage checks for app-owned billing composition.",
		"`cmd/provider-replay` validates checked-in provider fixtures locally and emits sanitized replay summaries.",
	} {
		if !strings.Contains(string(generatedREADME), want) {
			t.Fatalf("generated README missing provider docs %q:\n%s", want, generatedREADME)
		}
	}

	for path, wants := range map[string][]string{
		"cmd/provider-replay/main.go": {
			"maxProviderFixtureBytes",
			"provider fixture path must stay under testdata/providers",
			"summarizeProviderFixture",
			"tenant mismatch",
			"raw_payload",
		},
		"internal/providers/stripebilling/billing.go": {
			"compatbilling.CheckoutSessionRequest",
			"ParseWebhook",
			"tenant_mismatch",
			"stripe.checkout_session.create",
		},
		"internal/providers/resendemail/invitations.go": {
			"type NoopSender struct{}",
			"SendInvitation",
			"invitation.email.send",
			"recipient_domain",
		},
		"internal/providers/clerkwebhooks/webhooks.go": {
			"HMACVerifier",
			"X-Clerk-Signature",
			"tenant_mismatch",
			"clerk_webhook.",
		},
	} {
		generated, err := os.ReadFile(filepath.Join(serviceDir, path))
		if err != nil {
			t.Fatalf("read generated provider file %s: %v", path, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(generated), want) {
				t.Fatalf("generated %s missing %q:\n%s", path, want, generated)
			}
		}
	}

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated provider service tidy failed:\n%s\nerror: %v", output, err)
	}
	generatedMod, err := os.ReadFile(filepath.Join(serviceDir, "go.mod"))
	if err != nil {
		t.Fatalf("read generated go.mod: %v", err)
	}
	for _, notWant := range []string{"github.com/stripe/stripe-go", "github.com/resend/resend-go", "github.com/clerk/clerk-sdk-go"} {
		if strings.Contains(string(generatedMod), notWant) {
			t.Fatalf("generated app should not add provider SDK directly %q:\n%s", notWant, generatedMod)
		}
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated provider service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "provider-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated provider-check failed:\n%s\nerror: %v", output, err)
	}
}

func TestGenerateResourceAddsTenantScopedCRUDToFullProfile(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service failed: %s", out.String())
	}
	out.Reset()
	code = run(context.Background(), []string{
		"generate", "resource",
		"--dir", serviceDir,
		"--name", "project",
		"--plural", "projects",
		"--field", "status:string:enum=active|archived",
		"--field", "rank:int",
		"--filter", "status",
		"--sort", "name",
		"--admin",
		"--relationship", "owner:organizations",
		"--object-field", "attachment_key",
		"--tenant-scoped",
		"--crud",
		"--postgres",
		"--soft-delete",
		"--etag",
		"--audit",
		"--webhooks",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("generate resource failed: %s", out.String())
	}
	if !strings.Contains(out.String(), "generated resource projects") {
		t.Fatalf("generate resource output = %q", out.String())
	}
	for _, name := range []string{
		"internal/domain/project.go",
		"internal/app/projects.go",
		"internal/app/projects_test.go",
		"internal/adapters/postgres/projects.go",
		"internal/adapters/postgres/projects_test.go",
		"internal/httpapi/projects.go",
		"internal/httpapi/projects_test.go",
		"migrations/20260517000200_projects.up.sql",
		"migrations/20260517000200_projects.down.sql",
	} {
		if _, err := os.Stat(filepath.Join(serviceDir, name)); err != nil {
			t.Fatalf("expected generated resource file %s: %v", name, err)
		}
	}
	generatedManifest, err := os.ReadFile(filepath.Join(serviceDir, "api-toolkit.yaml"))
	if err != nil {
		t.Fatalf("read generated manifest after resource: %v", err)
	}
	for _, want := range []string{"name: project", "plural: projects", "tenant_scoped: true", "postgres: true", "soft_delete: true", "etag: true", "audit: true", "webhooks: true", "admin: true", "status:string:enum=active|archived", "rank:int", "filters:", "sorts:", "relationships:", "object_fields:"} {
		if !strings.Contains(string(generatedManifest), want) {
			t.Fatalf("generated manifest missing resource %q:\n%s", want, generatedManifest)
		}
	}
	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read router after resource: %v", err)
	}
	for _, want := range []string{"Projects *app.ProjectService", `r.Get("/projects"`, `r.Post("/projects"`, `registerPatch(r, "/projects/{id}"`, `r.Delete("/projects/{id}"`} {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated router missing resource wiring %q", want)
		}
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read main after resource: %v", err)
	}
	for _, want := range []string{"projects := app.NewProjectService()", "projects = app.NewProjectServiceWithStore(postgres.NewProjectStore(pool))", "Projects:"} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated main missing resource wiring %q", want)
		}
	}
	generatedOpenAPI, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "openapi.go"))
	if err != nil {
		t.Fatalf("read openapi after resource: %v", err)
	}
	for _, want := range []string{"ProjectCreateRequest", "ProjectList", "listProjects", "createProject", "updateProject", "deleteProject", `"status":`, `map[string]any{"type": "string", "enum": []string{"active", "archived"}}`, `"rank":`, `"type": "integer"`, `"owner_id":`, `map[string]any{"type": "string"}`, `"attachment_key":`, `map[string]any{"type": "string", "description": "Object key stored by object service"}`, `Name: "status", In: "query"`, `Name: "sort", In: "query"`} {
		if !strings.Contains(string(generatedOpenAPI), want) {
			t.Fatalf("generated openapi missing resource wiring %q", want)
		}
	}
	generatedDomain, err := os.ReadFile(filepath.Join(serviceDir, "internal", "domain", "project.go"))
	if err != nil {
		t.Fatalf("read generated domain resource: %v", err)
	}
	for _, want := range []string{"Status", "string", "Rank", "int", "OwnerID", "AttachmentKey", `"status":`, `r.Status`, `"rank":`, `r.Rank`, `"owner_id":`, `r.OwnerID`, `"attachment_key":`, `r.AttachmentKey`} {
		if !strings.Contains(string(generatedDomain), want) {
			t.Fatalf("generated domain missing resource field %q:\n%s", want, generatedDomain)
		}
	}
	generatedApp, err := os.ReadFile(filepath.Join(serviceDir, "internal", "app", "projects.go"))
	if err != nil {
		t.Fatalf("read generated app resource: %v", err)
	}
	for _, want := range []string{"type ProjectListOptions struct", "Filters map[string]string", "Sort", "matchProjectFilters", `case "status":`, `case "-name":`} {
		if !strings.Contains(string(generatedApp), want) {
			t.Fatalf("generated app missing resource list support %q:\n%s", want, generatedApp)
		}
	}
	generatedHTTP, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "projects.go"))
	if err != nil {
		t.Fatalf("read generated http resource: %v", err)
	}
	for _, want := range []string{"parseProjectListOptions", "projectFilterQueryParams", `filters["status"]`, `sort := strings.TrimSpace(query.Get("sort"))`, `case "name", "-name"`} {
		if !strings.Contains(string(generatedHTTP), want) {
			t.Fatalf("generated http resource missing list query support %q:\n%s", want, generatedHTTP)
		}
	}
	generatedPostgres, err := os.ReadFile(filepath.Join(serviceDir, "internal", "adapters", "postgres", "projects.go"))
	if err != nil {
		t.Fatalf("read generated postgres resource: %v", err)
	}
	for _, want := range []string{"type projectListQuery struct", "buildProjectListQuery", `case "status":`, `case "name":`, `case "-name":`, "status=$", "name asc, id asc"} {
		if !strings.Contains(string(generatedPostgres), want) {
			t.Fatalf("generated postgres resource missing list query support %q:\n%s", want, generatedPostgres)
		}
	}
	generatedMigration, err := os.ReadFile(filepath.Join(serviceDir, "migrations", "20260517000200_projects.up.sql"))
	if err != nil {
		t.Fatalf("read generated resource migration: %v", err)
	}
	for _, want := range []string{"status TEXT", "rank INTEGER", "owner_id TEXT", "attachment_key TEXT", "CHECK (status IN ('active', 'archived'))", "CREATE INDEX projects_organization_status_idx", "CREATE INDEX projects_organization_owner_id_idx", "CREATE INDEX projects_organization_attachment_key_idx"} {
		if !strings.Contains(string(generatedMigration), want) {
			t.Fatalf("generated migration missing resource field %q:\n%s", want, generatedMigration)
		}
	}
	cmd := exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated resource service tests failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "make", "resource-check")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated resource-check failed:\n%s\nerror: %v", output, err)
	}
}

func TestGenerateResourceFailsOutsideFullProfile(t *testing.T) {
	tmp := t.TempDir()
	var out strings.Builder
	code := run(context.Background(), []string{
		"generate", "resource",
		"--dir", tmp,
		"--name", "project",
		"--plural", "projects",
		"--tenant-scoped",
		"--crud",
	}, &out, &out)
	if code == 0 {
		t.Fatalf("generate resource unexpectedly succeeded: %s", out.String())
	}
	if !strings.Contains(out.String(), "api-toolkit.yaml") {
		t.Fatalf("generate resource error should mention manifest, got: %s", out.String())
	}
}

func TestGenerateResourceFailsClosedWhenAnchorsMissing(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service failed: %s", out.String())
	}
	routerPath := filepath.Join(serviceDir, "internal", "httpapi", "router.go")
	router, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("read router: %v", err)
	}
	router = []byte(strings.ReplaceAll(string(router), "// api-toolkit:router-register-routes", "// removed"))
	// #nosec G703 -- test path is inside t.TempDir and is intentionally mutated to simulate a dirty generated anchor.
	if err := os.WriteFile(routerPath, router, 0o600); err != nil {
		t.Fatalf("dirty router: %v", err)
	}
	out.Reset()
	code = run(context.Background(), []string{
		"generate", "resource",
		"--dir", serviceDir,
		"--name", "project",
		"--plural", "projects",
		"--tenant-scoped",
		"--crud",
	}, &out, &out)
	if code == 0 {
		t.Fatalf("generate resource unexpectedly succeeded: %s", out.String())
	}
	if !strings.Contains(out.String(), "generated anchors missing") {
		t.Fatalf("generate resource error should mention missing anchors, got: %s", out.String())
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithOIDC(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", "oidc",
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new OIDC full service failed: %s", out.String())
	}

	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full OIDC main.go: %v", err)
	}
	for _, want := range []string{"newOIDCMiddleware", "oidcauth.NewMiddleware", "OIDCJWKSURL", "OIDCDiscoveryURL"} {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full OIDC main.go missing %q", want)
		}
	}
	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full OIDC router.go: %v", err)
	}
	for _, want := range []string{"oidcauth.SubjectFromContext", "required OIDC scope missing", "tenant claim mismatch"} {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full OIDC router.go missing %q", want)
		}
	}
	generatedEnv, err := os.ReadFile(filepath.Join(serviceDir, ".env.example"))
	if err != nil {
		t.Fatalf("read generated full OIDC .env.example: %v", err)
	}
	for _, want := range []string{"OIDC_ISSUER=", "OIDC_AUDIENCE=saas-api-full", "OIDC_JWKS_URL=", "OIDC_DISCOVERY_URL=", "OIDC_TENANT_CLAIM=tenant_id", "OIDC_SCOPE_CLAIM=scope"} {
		if !strings.Contains(string(generatedEnv), want) {
			t.Fatalf("generated full OIDC .env.example missing %q", want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, "BearerAuth")

	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full OIDC service tidy failed:\n%s\nerror: %v", output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full OIDC service tests failed:\n%s\nerror: %v", output, err)
	}
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithJWT(t *testing.T) {
	assertFullScaffoldBearerAuthMode(t, "jwt", []string{"newJWTMiddleware", "jwtauth.NewMiddleware", "JWTJWKSURL", "JWTIssuer", "JWTAudience"}, []string{"jwtauth.SubjectFromContext", "required JWT scope missing", "tenant claim mismatch"}, "BearerAuth")
}

func TestNewServiceGeneratesBuildableSaaSAPIFullWithClerk(t *testing.T) {
	assertFullScaffoldBearerAuthMode(t, "clerk", []string{"newClerkMiddleware", "clerkauth.NewMiddleware", "ClerkJWKSURL", "ClerkIssuer", "ClerkAudience"}, []string{"clerkauth.SubjectFromContext", "required Clerk scope missing", "tenant claim mismatch"}, "BearerAuth")
}

func assertFullScaffoldBearerAuthMode(t *testing.T, authMode string, mainWant, routerWant []string, securityScheme string) {
	t.Helper()
	repoRoot := mustRepoRoot(t)
	tmp := t.TempDir()
	serviceDir := filepath.Join(tmp, "service")
	var out strings.Builder
	code := run(context.Background(), []string{
		"new", "service",
		"--module", "example.com/my-api",
		"--profile", "saas-api-full",
		"--auth", authMode,
		"--dir", serviceDir,
		"--core-replace", repoRoot,
		"--contrib-replace", filepath.Join(repoRoot, "contrib"),
	}, &out, &out)
	if code != 0 {
		t.Fatalf("new full service auth=%s failed: %s", authMode, out.String())
	}
	generatedMain, err := os.ReadFile(filepath.Join(serviceDir, "cmd", "api", "main.go"))
	if err != nil {
		t.Fatalf("read generated full %s main.go: %v", authMode, err)
	}
	for _, want := range mainWant {
		if !strings.Contains(string(generatedMain), want) {
			t.Fatalf("generated full %s main.go missing %q", authMode, want)
		}
	}
	generatedRouter, err := os.ReadFile(filepath.Join(serviceDir, "internal", "httpapi", "router.go"))
	if err != nil {
		t.Fatalf("read generated full %s router.go: %v", authMode, err)
	}
	for _, want := range routerWant {
		if !strings.Contains(string(generatedRouter), want) {
			t.Fatalf("generated full %s router.go missing %q", authMode, want)
		}
	}
	assertGeneratedGoldenHasGlobalSecurity(t, serviceDir, securityScheme)
	cmd := exec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = serviceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full %s service tidy failed:\n%s\nerror: %v", authMode, output, err)
	}
	cmd = exec.CommandContext(context.Background(), "go", "test", "./...")
	cmd.Dir = serviceDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated full %s service tests failed:\n%s\nerror: %v", authMode, output, err)
	}
}

func TestContractsLintFailsForMissingPolicy(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/widgets":{"post":{"responses":{"201":{"description":"created"}}}}}}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{
		"operation_id_required",
		"security_required",
		"unsafe_write_tenant_required",
		"unsafe_write_rate_limit_required",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintFailsForPrivateReadWithoutSecurity(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "security_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForPrivateReadWithoutProblemResponse(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "problem_response_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForUndefinedSecurityScheme(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {"description": "ok"},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"MissingAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "security_scheme_undefined") || !strings.Contains(errOut.String(), "MissingAuth") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintFailsForUndefinedGlobalSecurityScheme(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"security": [{"MissingGlobalAuth": []}],
		"paths": {}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"security_scheme_undefined", "GLOBAL", "MissingGlobalAuth"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintUsesGlobalSecurityRequirements(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"security": [{"ApiKeyAuth": ["widgets:read"]}],
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {
						"200": {"description": "ok"},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"type": "object"}}}
						}
					}
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &out, &out)
	if code != 0 {
		t.Fatalf("expected lint to pass with inherited global security: %s", out.String())
	}
}

func TestContractsLintFailsForUndocumentedUnsafeWrite(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"responses": {
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}],
				"x-tenant": {"required": true, "source": "header"},
				"x-idempotency-key": {"required": true},
				"x-rate-limit": "write-standard"
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"unsafe_write_request_body_required", "unsafe_write_success_response_required"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintFailsForNonRequiredUnsafeWritePolicies(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"deprecated": true,
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created"},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}],
				"x-tenant": {"required": false, "source": "header"},
				"x-idempotency-key": {"required": false, "header": "Idempotency-Key"},
				"x-rate-limit": "write-standard"
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{"unsafe_write_tenant_required", "unsafe_write_idempotency_required"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintAllowsPublicReadinessWithoutSecurity(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/readyz": {
			"get": {
				"operationId": "getReadiness",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &out, &out)
	if code != 0 {
		t.Fatalf("expected public readiness lint to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts lint passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsLintFailsForDuplicateOperationIDs(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/widgets": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		},
		"/widget-exports": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["exports:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	if !strings.Contains(errOut.String(), "operation_id_duplicate") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsLintAcceptsOpenAPI31NullableExamplesAndTypedClientAssumptions(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			},
			"schemas": {
				"Problem": {
					"type": "object",
					"properties": {
						"type": {"type": "string"},
						"title": {"type": "string"},
						"status": {"type": "integer"}
					}
				},
				"Widget": {
					"type": "object",
					"required": ["id", "name"],
					"properties": {
						"id": {"type": "string", "examples": ["wgt_1"]},
						"name": {"type": "string"},
						"description": {"type": ["string", "null"], "examples": ["demo"]}
					}
				},
				"WidgetList": {
					"type": "object",
					"required": ["items"],
					"properties": {
						"items": {"type": "array", "items": {"$ref": "#/components/schemas/Widget"}},
						"next_cursor": {"type": ["string", "null"], "examples": ["cursor_1"]}
					}
				}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"parameters": [
						{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string", "examples": ["tenant_1"]}},
						{"name": "cursor", "in": "query", "required": false, "schema": {"type": ["string", "null"], "examples": ["cursor_1"]}}
					],
					"responses": {
						"200": {
							"description": "ok",
							"content": {
								"application/json": {
									"schema": {"$ref": "#/components/schemas/WidgetList"},
									"examples": {"ok": {"value": {"items": []}}}
								}
							}
						},
						"400": {
							"description": "bad request",
							"content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}
						}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &out, &out)
	if code != 0 {
		t.Fatalf("expected OpenAPI 3.1 lint to pass: %s", out.String())
	}
	clientDir := filepath.Join(tmp, "client")
	out.Reset()
	code = run(context.Background(), []string{
		"clients", "go",
		"--openapi", specPath,
		"--out", clientDir,
		"--package", "apiclient",
		"--style", "typed",
	}, &out, &out)
	if code != 0 {
		t.Fatalf("expected typed client generation to pass: %s", out.String())
	}
	generated, err := os.ReadFile(filepath.Join(clientDir, "client.go"))
	if err != nil {
		t.Fatalf("read generated client: %v", err)
	}
	for _, want := range []string{
		"Description *string `json:\"description,omitempty\"`",
		"NextCursor *string  `json:\"next_cursor,omitempty\"`",
		"type ListWidgetsParams struct",
		"Cursor    *string",
		"XTenantID string",
		"func (c *Client) ListWidgets(ctx context.Context, params ListWidgetsParams, opts ...RequestOption) (*WidgetList, *http.Response, error)",
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated typed client missing %q:\n%s", want, generated)
		}
	}
}

func TestContractsLintFailsForGoClientIdentifierConflicts(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			},
			"schemas": {
				"Problem": {"type": "object"},
				"Widget": {"type": "object", "properties": {"id": {"type": "string"}}},
				"widget": {"type": "object", "properties": {"id": {"type": "string"}}}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "get-widget",
					"parameters": [
						{"name": "X-Tenant-ID", "in": "header", "required": true, "schema": {"type": "string"}},
						{"name": "x_tenant_id", "in": "query", "required": false, "schema": {"type": "string"}}
					],
					"responses": {
						"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			},
			"/widget-exports": {
				"get": {
					"operationId": "get_widget",
					"responses": {
						"200": {"description": "ok"},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail")
	}
	for _, want := range []string{
		"go_client_method_id_conflict",
		"go_client_schema_id_conflict",
		"go_client_parameter_id_conflict",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsLintAllowsAdditionalPublicPath(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/status": {
			"get": {
				"operationId": "getStatus",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail without custom public path")
	}
	if !strings.Contains(errOut.String(), "security_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}

	var out strings.Builder
	code = run(context.Background(), []string{"contracts", "lint", "--openapi", specPath, "--public-path", "/status"}, &out, &out)
	if code != 0 {
		t.Fatalf("expected custom public path lint to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts lint passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsLintChecksAdditionalAdminPath(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	writeTestOpenAPI(t, specPath, `{
		"/internal/debug": {
			"get": {
				"operationId": "getInternalDebug",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["admin:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath, "--admin-path", "/internal/debug"}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected lint to fail for custom admin path without policy")
	}
	if !strings.Contains(errOut.String(), "admin_policy_required") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestContractsDiffAllowsAdditiveOperations(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}}
			}
		},
		"/widget-exports": {
			"get": {
				"operationId": "getWidget",
				"responses": {"200": {"description": "ok"}, "404": {"description": "not found"}}
			}
		}
	}`)

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &out, &out)
	if code != 0 {
		t.Fatalf("expected additive diff to pass: %s", out.String())
	}
	if !strings.Contains(out.String(), "contracts diff passed") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestContractsChangelogAndImpactReportClientChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			},
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		},
		"/projects": {
			"get": {
				"operationId": "listProjects",
				"responses": {
					"200": {"description": "ok", "content": {"application/json": {"schema": {"type": "object"}}}},
					"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"type": "object"}}}}
				},
				"security": [{"ApiKeyAuth": ["projects:read"]}]
			}
		}
	}`)

	var changelog strings.Builder
	code := run(context.Background(), []string{"contracts", "changelog", "--base", base, "--head", head}, &changelog, &changelog)
	if code != 0 {
		t.Fatalf("contracts changelog failed: %s", changelog.String())
	}
	for _, want := range []string{
		"# OpenAPI Changelog",
		"Added operations",
		"GET /projects listProjects",
		"Removed operations",
		"POST /widgets createWidget",
	} {
		if !strings.Contains(changelog.String(), want) {
			t.Fatalf("changelog missing %q:\n%s", want, changelog.String())
		}
	}

	var impact strings.Builder
	code = run(context.Background(), []string{"contracts", "impact", "--base", base, "--head", head}, &impact, &impact)
	if code != 1 {
		t.Fatalf("contracts impact exit code = %d output=%s", code, impact.String())
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(impact.String()), &report); err != nil {
		t.Fatalf("decode impact report: %v\n%s", err, impact.String())
	}
	if breaking, _ := report["breaking"].(bool); !breaking {
		t.Fatalf("impact report should be breaking: %#v", report)
	}
	removed, _ := report["removed_operations"].([]any)
	if len(removed) != 1 {
		t.Fatalf("impact report removed operations = %#v", report["removed_operations"])
	}
}

func TestContractsLintRequiresOpenAPI31FeatureMetadata(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.json")
	spec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			},
			"schemas": {
				"Problem": {
					"type": "object",
					"properties": {"title": {"type": "string"}, "status": {"type": "integer"}}
				},
				"Event": {
					"oneOf": [
						{"type": "object", "properties": {"kind": {"type": "string"}}}
					]
				}
			}
		},
		"paths": {
			"/exports": {
				"get": {
					"operationId": "getExport",
					"responses": {
						"200": {"description": "ok", "content": {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["exports:read"]}]
				}
			},
			"/events": {
				"get": {
					"operationId": "streamEvents",
					"responses": {
						"200": {"description": "ok", "content": {"text/event-stream": {"schema": {"type": "string"}}}},
						"400": {"description": "bad request", "content": {"application/problem+json": {"schema": {"$ref": "#/components/schemas/Problem"}}}}
					},
					"security": [{"ApiKeyAuth": ["events:read"]}]
				}
			}
		}
	}`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "lint", "--openapi", specPath}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected contracts lint to fail")
	}
	for _, want := range []string{
		"client_schema_composition_review_required",
		"binary_response_metadata_required",
		"streaming_metadata_required",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsImpactReportsSchemaDefaultsEnumsAndComposition(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	baseSpec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Widget": {
					"x-api-toolkit-client-compatible": true,
					"oneOf": [{"$ref": "#/components/schemas/WidgetA"}],
					"properties": {
						"status": {"type": "string", "enum": ["active", "archived"], "default": "active"}
					}
				},
				"WidgetA": {"type": "object"},
				"WidgetB": {"type": "object"}
			}
		},
		"paths": {}
	}`
	headSpec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Widget": {
					"x-api-toolkit-client-compatible": true,
					"oneOf": [{"$ref": "#/components/schemas/WidgetA"}, {"$ref": "#/components/schemas/WidgetB"}],
					"properties": {
						"status": {"type": "string", "enum": ["active", "beta"], "default": "beta"}
					}
				},
				"WidgetA": {"type": "object"},
				"WidgetB": {"type": "object"}
			}
		},
		"paths": {}
	}`
	if err := os.WriteFile(base, []byte(baseSpec), 0o600); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if err := os.WriteFile(head, []byte(headSpec), 0o600); err != nil {
		t.Fatalf("write head: %v", err)
	}

	var impact strings.Builder
	code := run(context.Background(), []string{"contracts", "impact", "--base", base, "--head", head}, &impact, &impact)
	if code != 1 {
		t.Fatalf("contracts impact exit code = %d output=%s", code, impact.String())
	}
	for _, want := range []string{
		"schema_default_changed",
		"schema_enum_value_added",
		"schema_enum_value_removed",
		"schema_composition_changed",
	} {
		if !strings.Contains(impact.String(), want) {
			t.Fatalf("impact missing %q:\n%s", want, impact.String())
		}
	}
}

func TestContractsDiffFailsForBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {"200": {"description": "ok"}, "401": {"description": "unauthorized"}},
				"security": [{"ApiKeyAuth": []}]
			},
			"post": {
				"operationId": "createWidget",
				"responses": {"201": {"description": "created"}, "400": {"description": "bad request"}}
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgetsRenamed",
				"responses": {"200": {"description": "ok"}}
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected breaking diff to fail")
	}
	for _, want := range []string{
		"operation_removed POST /widgets",
		"operation_id_changed GET /widgets",
		"response_removed GET /widgets",
		"security_changed GET /widgets",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForGlobalSecurityDrift(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	baseSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"security": [{"ApiKeyAuth": ["widgets:read"]}],
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`
	headSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`
	if err := os.WriteFile(base, []byte(baseSpec), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(headSpec), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected global security diff to fail")
	}
	for _, want := range []string{
		"global_security_changed",
		"security_changed GET /widgets",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForRoutePolicyDrift(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created"},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}],
				"x-tenant": {"required": true, "source": "header"},
				"x-idempotency-key": {"required": true, "header": "Idempotency-Key"},
				"x-rate-limit": "write-standard",
				"x-sunset": "Wed, 01 Jul 2026 00:00:00 GMT"
			}
		},
		"/metrics": {
			"get": {
				"operationId": "getMetrics",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["admin:read"]}],
				"x-admin-policy": "admin"
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {
					"201": {"description": "created"},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}],
				"x-tenant": {"required": false, "source": "path"},
				"x-rate-limit": "write-burst"
			}
		},
		"/metrics": {
			"get": {
				"operationId": "getMetrics",
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["admin:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected policy drift diff to fail")
	}
	for _, want := range []string{
		"tenant_policy_changed POST /widgets",
		"idempotency_policy_changed POST /widgets",
		"rate_limit_policy_changed POST /widgets",
		"deprecation_policy_changed POST /widgets",
		"admin_policy_changed GET /metrics",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForRequestBodyBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": false,
					"content": {
						"application/json": {"schema": {"type": "object"}},
						"application/vnd.widgets+json": {"schema": {"type": "object"}}
					}
				},
				"responses": {"201": {"description": "created"}},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		},
		"/widget-imports": {
			"post": {
				"operationId": "importWidgets",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {"202": {"description": "accepted"}},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {"application/json": {"schema": {"type": "object"}}}
				},
				"responses": {"201": {"description": "created"}},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		},
		"/widget-imports": {
			"post": {
				"operationId": "importWidgets",
				"responses": {"202": {"description": "accepted"}},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected request body breaking diff to fail")
	}
	for _, want := range []string{
		"request_body_required_added POST /widgets",
		"request_body_content_removed POST /widgets application/vnd.widgets+json",
		"request_body_removed POST /widget-imports",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForResponseContentBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {
						"description": "ok",
						"content": {
							"application/json": {"schema": {"type": "object"}},
							"application/vnd.widgets+json": {"schema": {"type": "object"}}
						}
					},
					"400": {
						"description": "bad request",
						"content": {"application/problem+json": {"schema": {"type": "object"}}}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"responses": {
					"200": {
						"description": "ok",
						"content": {"application/json": {"schema": {"type": "object"}}}
					},
					"400": {"description": "bad request"}
				},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected response content breaking diff to fail")
	}
	for _, want := range []string{
		"response_content_removed GET /widgets 200 application/vnd.widgets+json",
		"response_content_removed GET /widgets 400 application/problem+json",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForParameterBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"parameters": [
					{"name": "cursor", "in": "query", "required": false, "schema": {"type": "string"}},
					{"name": "filter", "in": "query", "required": false, "schema": {"type": "string"}}
				],
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"get": {
				"operationId": "listWidgets",
				"parameters": [
					{"name": "filter", "in": "query", "required": true, "schema": {"type": "string"}},
					{"name": "X-Client-Version", "in": "header", "required": true, "schema": {"type": "string"}},
					{"name": "expand", "in": "query", "required": false, "schema": {"type": "string"}}
				],
				"responses": {"200": {"description": "ok"}},
				"security": [{"ApiKeyAuth": ["widgets:read"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected parameter breaking diff to fail")
	}
	for _, want := range []string{
		"parameter_removed GET /widgets query:cursor",
		"parameter_required_added GET /widgets query:filter",
		"required_parameter_added GET /widgets header:X-Client-Version",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForComponentSchemaBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	if err := os.WriteFile(base, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Legacy": {"type": "object", "properties": {"id": {"type": "string"}}},
				"Widget": {
					"type": "object",
					"required": ["name"],
					"properties": {
						"name": {"type": "string"},
						"status": {"type": "string", "enum": ["active", "disabled"]},
						"tenant_id": {"type": "string"}
					}
				}
			},
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Widget": {
					"type": "object",
					"required": ["name", "status"],
					"properties": {
						"name": {"type": "integer"},
						"status": {"type": "string", "enum": ["active"]}
					}
				}
			},
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected schema breaking diff to fail")
	}
	for _, want := range []string{
		"schema_removed Legacy",
		"schema_required_property_added Widget status",
		"schema_property_removed Widget tenant_id",
		"schema_type_changed Widget.name",
		"schema_enum_value_removed Widget.status \"disabled\"",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffHandlesOpenAPI31NullableAndExamples(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	examplesOnly := filepath.Join(tmp, "examples-only.json")
	nullableRemoved := filepath.Join(tmp, "nullable-removed.json")
	baseSpec := `{
		"openapi": "3.1.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"schemas": {
				"Widget": {
					"type": "object",
					"properties": {
						"id": {"type": "string", "examples": ["wgt_1"]},
						"description": {"type": ["string", "null"], "examples": ["old"]}
					}
				}
			},
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`
	examplesOnlySpec := strings.ReplaceAll(baseSpec, `"examples": ["old"]`, `"examples": ["new"]`)
	nullableRemovedSpec := strings.ReplaceAll(examplesOnlySpec, `"type": ["string", "null"], "examples": ["new"]`, `"type": "string", "examples": ["new"]`)
	if err := os.WriteFile(base, []byte(baseSpec), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(examplesOnly, []byte(examplesOnlySpec), 0o600); err != nil {
		t.Fatalf("write examples-only spec: %v", err)
	}
	if err := os.WriteFile(nullableRemoved, []byte(nullableRemovedSpec), 0o600); err != nil {
		t.Fatalf("write nullable-removed spec: %v", err)
	}

	var out strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", examplesOnly}, &out, &out)
	if code != 0 {
		t.Fatalf("expected examples-only diff to pass: %s", out.String())
	}

	var errOut strings.Builder
	code = run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", nullableRemoved}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected nullable removal diff to fail")
	}
	if !strings.Contains(errOut.String(), "schema_type_changed Widget.description") {
		t.Fatalf("stderr missing nullable schema diff:\n%s", errOut.String())
	}
}

func TestContractsDiffFailsForSecuritySchemeBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	if err := os.WriteFile(base, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"},
				"WebhookAuth": {"type": "apiKey", "in": "header", "name": "X-Webhook-Signature"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write base spec: %v", err)
	}
	if err := os.WriteFile(head, []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key-V2"}
			}
		},
		"paths": {
			"/widgets": {
				"get": {
					"operationId": "listWidgets",
					"responses": {"200": {"description": "ok"}},
					"security": [{"ApiKeyAuth": ["widgets:read"]}]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write head spec: %v", err)
	}

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected security scheme breaking diff to fail")
	}
	for _, want := range []string{
		"security_scheme_changed ApiKeyAuth",
		"security_scheme_removed WebhookAuth",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func TestContractsDiffFailsForInlineSchemaBreakingChanges(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.json")
	head := filepath.Join(tmp, "head.json")
	writeTestOpenAPI(t, base, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {
						"application/json": {
							"schema": {
								"type": "object",
								"required": ["name"],
								"properties": {
									"name": {"type": "string"},
									"status": {"type": "string", "enum": ["active", "disabled"]}
								}
							}
						}
					}
				},
				"responses": {
					"201": {
						"description": "created",
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"id": {"type": "string"},
										"status": {"type": "string"}
									}
								}
							}
						}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		}
	}`)
	writeTestOpenAPI(t, head, `{
		"/widgets": {
			"post": {
				"operationId": "createWidget",
				"requestBody": {
					"required": true,
					"content": {
						"application/json": {
							"schema": {
								"type": "object",
								"required": ["name", "status"],
								"properties": {
									"name": {"type": "integer"},
									"status": {"type": "string", "enum": ["active"]}
								}
							}
						}
					}
				},
				"responses": {
					"201": {
						"description": "created",
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"id": {"type": "integer"}
									}
								}
							}
						}
					}
				},
				"security": [{"ApiKeyAuth": ["widgets:write"]}]
			}
		}
	}`)

	var errOut strings.Builder
	code := run(context.Background(), []string{"contracts", "diff", "--base", base, "--head", head}, &strings.Builder{}, &errOut)
	if code == 0 {
		t.Fatal("expected schema breaking diff to fail")
	}
	for _, want := range []string{
		"schema_required_property_added POST /widgets requestBody application/json status",
		"schema_type_changed POST /widgets requestBody application/json.name",
		"schema_enum_value_removed POST /widgets requestBody application/json.status \"disabled\"",
		"schema_type_changed POST /widgets response 201 application/json.id",
		"schema_property_removed POST /widgets response 201 application/json status",
	} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut.String())
		}
	}
}

func writeTestOpenAPI(t *testing.T, path, pathsJSON string) {
	t.Helper()
	spec := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1"},
		"components": {
			"securitySchemes": {
				"ApiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
			}
		},
		"paths": ` + pathsJSON + `
	}`
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

func assertGeneratedGoldenHasGlobalSecurity(t *testing.T, serviceDir, scheme string) {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join(serviceDir, "testdata", "openapi.golden.json"))
	if err != nil {
		t.Fatalf("read generated OpenAPI golden: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(golden, &doc); err != nil {
		t.Fatalf("decode generated OpenAPI golden: %v", err)
	}
	security, ok := doc["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatalf("generated OpenAPI golden has no top-level security: %s", golden)
	}
	for _, entry := range security {
		requirement, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := requirement[scheme]; ok {
			return
		}
	}
	t.Fatalf("generated OpenAPI golden missing top-level security scheme %q: %s", scheme, golden)
}

func assertGeneratedGoldenOpenAPIVersion(t *testing.T, serviceDir, version string) {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join(serviceDir, "testdata", "openapi.golden.json"))
	if err != nil {
		t.Fatalf("read generated OpenAPI golden: %v", err)
	}
	var doc struct {
		OpenAPI string `json:"openapi"`
	}
	if err := json.Unmarshal(golden, &doc); err != nil {
		t.Fatalf("decode generated OpenAPI golden: %v", err)
	}
	if doc.OpenAPI != version {
		t.Fatalf("generated OpenAPI version = %q, want %q", doc.OpenAPI, version)
	}
}

func assertGeneratedREADMEListsOperatorRoutes(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{"`GET /health/detailed` with `X-Admin-Key`", "`GET /metrics` with `X-Admin-Key`", "`GET /debug/pprof/` with `X-Admin-Key`"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing operator route %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsIdempotencyRequirement(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"Unsafe writes without `Idempotency-Key` fail with Problem Details 400",
		"Idempotency storage keys are tenant and actor scoped",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing idempotency guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsRateLimitStore(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"Local development uses `RATE_LIMIT_STORE=memory`",
		"In production, the generated service defaults to `RATE_LIMIT_STORE=redis`",
		"requires `RATE_LIMIT_REDIS_ADDR` or `REDIS_ADDR`",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing rate-limit store guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedREADMEDocumentsTelemetry(t *testing.T, readme string) {
	t.Helper()
	for _, want := range []string{
		"OpenTelemetry tracing is disabled by default with `OTEL_TRACING_ENABLED=false`",
		"`OTEL_EXPORTER_OTLP_ENDPOINT` is required when tracing is enabled",
		"tracer provider is closed through the service shutdown hooks",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("generated README missing telemetry guidance %q:\n%s", want, readme)
		}
	}
}

func assertGeneratedEnvDocumentsRateLimitConfig(t *testing.T, env string) {
	t.Helper()
	for _, want := range []string{
		"TRUSTED_PROXIES=",
		"RATE_LIMIT_SKIP_ENABLED=false",
		"RATE_LIMIT_SKIP_HEADER=",
		"RATE_LIMIT_ALLOW_DANGEROUS_DEV_BYPASSES=false",
		"RATE_LIMIT_STORE=memory",
		"RATE_LIMIT_REDIS_ADDR=",
		"RATE_LIMIT_KEY_PREFIX=ratelimit:",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("generated .env.example missing rate-limit config %q:\n%s", want, env)
		}
	}
}

func assertGeneratedEnvDocumentsTelemetryConfig(t *testing.T, env string) {
	t.Helper()
	for _, want := range []string{
		"OTEL_TRACING_ENABLED=false",
		"OTEL_SERVICE_NAME=api",
		"OTEL_EXPORTER_OTLP_ENDPOINT=",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
		"OTEL_TRACES_SAMPLER=parentbased_traceidratio",
		"OTEL_SAMPLE_RATIO=1",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("generated .env.example missing telemetry config %q:\n%s", want, env)
		}
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root from %s: %v", wd, err)
	}
	return root
}
