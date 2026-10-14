// Package kerneltest supplies privileged test infrastructure only.
package kerneltest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Preflight(ctx context.Context) (string, error) {
	return preflight(ctx, func(ctx context.Context) error {
		// Probe in a disposable child; never mutate the caller's namespace.
		out, err := exec.CommandContext(ctx, "unshare", "--net", "true").CombinedOutput()
		if err != nil {
			return fmt.Errorf("isolated network namespace: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	})
}
func preflight(ctx context.Context, probe func(context.Context) error) (string, error) {
	if os.Getenv("PICKETX_KERNEL_TESTS") != "1" {
		return "", fmt.Errorf("kernel tests require PICKETX_KERNEL_TESTS=1")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := probe(ctx); err != nil {
		return "", fmt.Errorf("kernel preflight: %w", err)
	}
	release, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "", fmt.Errorf("kernel release: %w", err)
	}
	return strings.TrimSpace(string(release)), nil
}
