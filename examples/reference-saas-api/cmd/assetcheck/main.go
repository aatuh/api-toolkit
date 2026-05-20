package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: assetcheck observability|deploy|all")
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	switch args[0] {
	case "observability":
		return checkObservability(root)
	case "deploy":
		return checkDeploy(root)
	case "all":
		if err := checkObservability(root); err != nil {
			return err
		}
		return checkDeploy(root)
	default:
		return fmt.Errorf("unknown asset check %q", args[0])
	}
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "api-toolkit.yaml")); err == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", errors.New("api-toolkit.yaml not found")
		}
		dir = next
	}
}

func checkObservability(root string) error {
	dashboardPath := filepath.Join(root, "observability/grafana/saas-api-full-dashboard.json")
	dashboard, err := os.ReadFile(dashboardPath)
	if err != nil {
		return err
	}
	var parsed struct {
		Title  string            `json:"title"`
		Panels []json.RawMessage `json:"panels"`
	}
	if err := json.Unmarshal(dashboard, &parsed); err != nil {
		return fmt.Errorf("%s: invalid JSON: %w", dashboardPath, err)
	}
	if parsed.Title == "" || len(parsed.Panels) == 0 {
		return fmt.Errorf("%s: dashboard must have title and panels", dashboardPath)
	}
	if strings.Contains(string(dashboard), "tenant_id") {
		return fmt.Errorf("%s: dashboard must not use tenant_id as a metric label", dashboardPath)
	}

	rulesPath := filepath.Join(root, "observability/prometheus/saas-api-full-rules.yaml")
	rules, err := os.ReadFile(rulesPath)
	if err != nil {
		return err
	}
	for _, want := range []string{"groups:", "ApiHighErrorRate", "ApiReadinessFailing", "OutboxBacklogGrowing", "WebhookDeliveryDeadLetters", "DependencyHealthFailing"} {
		if !strings.Contains(string(rules), want) {
			return fmt.Errorf("%s: missing %q", rulesPath, want)
		}
	}
	if strings.Contains(string(rules), "tenant_id") {
		return fmt.Errorf("%s: rules must not use tenant_id as a metric label", rulesPath)
	}
	return nil
}

func checkDeploy(root string) error {
	required := map[string][]string{
		"deploy/helm/Chart.yaml":                   {"apiVersion: v2", "api-toolkit-service"},
		"deploy/helm/values.yaml":                  {"livez: /livez", "readyz: /readyz", "adminService:"},
		"deploy/kubernetes/deployment.yaml":        {"path: /livez", "path: /readyz", "runAsNonRoot: true"},
		"deploy/kubernetes/worker-deployment.yaml": {"kind: Deployment", "app: api-worker"},
		"deploy/kubernetes/migration-job.yaml":     {"kind: Job", "migrate"},
		"deploy/kubernetes/admin-service.yaml":     {"internal-only", "name: admin"},
		"deploy/kubernetes/network-policy.yaml":    {"kind: NetworkPolicy"},
		"deploy/terraform/aws/main.tf":             {"aws_db_instance", "aws_elasticache_replication_group", "aws_s3_bucket", "aws_iam_policy"},
		"deploy/terraform/aws/variables.tf":        {"variable \"name\"", "variable \"postgres_password\""},
		"deploy/terraform/aws/outputs.tf":          {"output \"database_endpoint\"", "output \"redis_endpoint\"", "output \"object_bucket_name\""},
	}
	for rel, wants := range required {
		path := filepath.Join(root, rel)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				return fmt.Errorf("%s: missing %q", path, want)
			}
		}
		if strings.HasSuffix(path, ".tf") && !balancedBraces(text) {
			return fmt.Errorf("%s: unbalanced braces", path)
		}
	}
	return nil
}

func balancedBraces(text string) bool {
	depth := 0
	for _, r := range text {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
