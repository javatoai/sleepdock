//go:build !windows && !darwin

package platform

import (
	"context"
	"fmt"
)

type Power struct{}

func (Power) Sleep(context.Context) error {
	return fmt.Errorf("此版本仅在 Windows 和 macOS 提供睡眠功能")
}
