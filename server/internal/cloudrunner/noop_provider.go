package cloudrunner

import (
	"context"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/cloudrunner/provider"
)

type noopProvider struct {
	logger *slog.Logger
}

func NewNoopProvider(logger *slog.Logger) provider.Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &noopProvider{logger: logger.With("provider", "noop")}
}

func (p *noopProvider) Start(context.Context) error { return nil }
func (p *noopProvider) Stop(context.Context) error  { return nil }

func (p *noopProvider) ExecuteTask(_ context.Context, task provider.Task) (provider.Result, error) {
	p.logger.Info("claimed task (noop)", "task_id", task.ID, "runtime_id", task.RuntimeID)
	return provider.Result{}, nil
}
