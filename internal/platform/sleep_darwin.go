//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Power struct{}

func (Power) Sleep(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "/usr/bin/pmset", "sleepnow").CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("macOS 未接受睡眠请求：%s", detail)
	}
	return nil
}
