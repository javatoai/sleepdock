package desktop

import (
	"fmt"
	"sleepdock/internal/domain"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPresetAndStart(t *testing.T) {
	v := Preview("idle")
	b := v.Backend.(*previewBackend)
	tt := ui.NewTester(v.Render, 560, 590)
	if err := tt.Click("30分"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始计时"); err != nil {
		t.Fatal(err)
	}
	if b.starts != 1 || b.model.Timer.Spec.Minutes != 30 || b.model.Timer.Phase != domain.Running {
		t.Fatal(b.model.Timer)
	}
	if tt.HasText("开始计时") || !tt.HasText("取消") {
		t.Fatal(tt.Texts())
	}
}
func TestAtTimeAndInvalidInput(t *testing.T) {
	v := Preview("at")
	b := v.Backend.(*previewBackend)
	tt := ui.NewTester(v.Render, 560, 590)
	if !tt.HasText("明天 02:01 睡眠") {
		t.Fatal(tt.Texts())
	}
	v.atHour = "99"
	tt.Frame()
	if err := tt.Click("开始计时"); err != nil {
		t.Fatal(err)
	}
	if b.starts != 0 {
		t.Fatal("invalid time started")
	}
	v.atHour = "02"
	tt.Frame()
	if err := tt.Click("开始计时"); err != nil {
		t.Fatal(err)
	}
	if b.model.Timer.Spec.Mode != "at" {
		t.Fatal(b.model.Timer.Spec)
	}
}
func TestCancelPostponeAndRetry(t *testing.T) {
	v := Preview("running")
	b := v.Backend.(*previewBackend)
	hidden := false
	v.Hide = func() { hidden = true }
	tt := ui.NewTester(v.Render, 560, 590)
	if err := tt.Click("收进托盘"); err != nil || !hidden {
		t.Fatal("tray action unavailable", err)
	}
	old := b.model.Timer.TriggerAt
	if err := tt.Click("延后15分钟"); err != nil {
		t.Fatal(err)
	}
	if !b.model.Timer.TriggerAt.After(old) {
		t.Fatal("not postponed")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if b.model.Timer.Phase != domain.Cancelled || !tt.HasText("重新设置") {
		t.Fatal(tt.Texts())
	}
	v = Preview("error")
	tt = ui.NewTester(v.Render, 560, 590)
	if tt.HasText("系统未接受睡眠请求。请检查本机电源权限。") {
		t.Fatal("details shown without asking")
	}
	if err := tt.Click("详情"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("系统未接受睡眠请求。请检查本机电源权限。") {
		t.Fatal("missing error details")
	}
}
func TestResponsiveLightTheme(t *testing.T) {
	for _, state := range []struct{ name, text string }{
		{"idle", "开始计时"}, {"at", "开始计时"}, {"running", "计时中"},
		{"grace", "即将睡眠"}, {"completed", "已结束"}, {"error", "未能睡眠"},
	} {
		for _, size := range [][2]int{{560, 590}, {480, 590}} {
			t.Run(fmt.Sprintf("%s/%dx%d", state.name, size[0], size[1]), func(t *testing.T) {
				v := Preview(state.name)
				tt := ui.NewTester(v.Render, size[0], size[1])
				if !tt.HasText(state.text) {
					t.Fatal(tt.Texts())
				}
			})
		}
	}
	v := Preview("idle")
	tt := ui.NewTester(v.Render, 560, 590)
	color := tt.Image().At(0, 0)
	tt.SetDark(true)
	if tt.Image().At(0, 0) != color {
		t.Fatal("system appearance changed the fixed light theme")
	}
}
func TestGraceDisplaysSecondsAndDirectActions(t *testing.T) {
	v := Preview("grace")
	tt := ui.NewTester(v.Render, 560, 590)
	if !tt.HasText("42") || !tt.HasText("即将睡眠") || !tt.HasText("取消") || !tt.HasText("延后15分钟") {
		t.Fatal(tt.Texts())
	}
}
