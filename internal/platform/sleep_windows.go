//go:build windows

package platform

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Power struct{}

func (Power) Sleep(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf("读取电源操作权限：%w", err)
	}
	defer token.Close()
	name, err := windows.UTF16PtrFromString("SeShutdownPrivilege")
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return err
	}
	privileges := windows.Tokenprivileges{PrivilegeCount: 1}
	privileges.Privileges[0].Luid = luid
	privileges.Privileges[0].Attributes = windows.SE_PRIVILEGE_ENABLED
	var old windows.Tokenprivileges
	var size uint32
	if err := windows.AdjustTokenPrivileges(token, false, &privileges, uint32(unsafe.Sizeof(old)), &old, &size); err != nil {
		return fmt.Errorf("申请电源操作权限：%w", err)
	}
	defer windows.AdjustTokenPrivileges(token, false, &old, 0, nil, nil)
	if err := ctx.Err(); err != nil {
		return err
	}
	// Hibernate=false, ForceCritical=false, DisableWakeEvent=false, matching
	// the user's original ordinary-suspend PowerShell behavior.
	result, _, callErr := windows.NewLazySystemDLL("powrprof.dll").NewProc("SetSuspendState").Call(0, 0, 0)
	if result == 0 {
		if callErr == nil || callErr == windows.ERROR_SUCCESS {
			return fmt.Errorf("Windows 未接受睡眠请求，请检查系统电源设置")
		}
		return fmt.Errorf("Windows 未接受睡眠请求：%w", callErr)
	}
	return nil
}
