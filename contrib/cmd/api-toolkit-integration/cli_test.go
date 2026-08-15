package cliintegration

import (
	"context"
	"os"
	"os/exec"
)

// runCLI exercises the independently versioned generator through its command
// boundary. Contrib owns the real-service harnesses because its internal test
// packages must not become dependencies of the installed CLI module.
func runCLI(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "../../../cmd/api-toolkit"}, args...)...)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	return cmd.CombinedOutput()
}
