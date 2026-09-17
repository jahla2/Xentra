package application

import (
	"context"
	"os/exec"
)

type OSExecutor struct{}
func (OSExecutor) Run(ctx context.Context, name string, args ...string) (string, error) { out, err := exec.CommandContext(ctx, name, args...).CombinedOutput(); return string(out), err }
