// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/PicketX/PicketXD/internal/agentcli"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if e := agentcli.New(os.Stdout, logger).ExecuteContext(ctx); e != nil && !errors.Is(e, context.Canceled) {
		logger.Error("picketxd stopped", "error", e)
		os.Exit(1)
	}
}
