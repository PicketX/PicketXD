package kerneltest

import (
	"context"
	"errors"
	"testing"
)

// Ignoring namespace-probe errors would falsely approve a privileged run.
func TestPreflightMissingPrivilege(t *testing.T) {
	t.Setenv("PICKETX_KERNEL_TESTS", "1")
	denied := errors.New("namespace permission denied")
	kernel, err := preflight(context.Background(), func(context.Context) error { return denied })
	if !errors.Is(err, denied) || kernel != "" {
		t.Fatalf("kernel=%q err=%v; require failed preflight", kernel, err)
	}
}
func TestPreflightRequiresOptIn(t *testing.T) {
	t.Setenv("PICKETX_KERNEL_TESTS", "")
	called := false
	_, err := preflight(context.Background(), func(context.Context) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("no opt-in: err=%v probe called=%v", err, called)
	}
}
