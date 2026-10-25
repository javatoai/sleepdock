package desktop

import (
	"fmt"
	"math"
	"sleepdock/internal/application"
	"sleepdock/internal/domain"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

type Backend interface {
	Snapshot() application.Model
	Start(domain.Spec)
	Postpone()
	Cancel()
	Retry()
	Reset()
	Dismiss()
}

type View struct {
	Backend                          Backend
	Hide                             func()
	mode                             int
	hours, minutes, atHour, atMinute string
	details                          bool
}

func NewView(backend Backend) *View {
	spec := backend.Snapshot().Preferences.Last
	v := &View{Backend: backend, hours: fmt.Sprintf("%02d", spec.Minutes/60), minutes: fmt.Sprintf("%02d", spec.Minutes%60), atHour: fmt.Sprintf("%02d", spec.Hour), atMinute: fmt.Sprintf("%02d", spec.Minute)}
	if spec.Mode == "at" {
		v.mode = 1
	}
	return v
}

func (v *View) spec() (domain.Spec, error) {
	h, m := v.hours, v.minutes
	mode := "duration"
	if v.mode == 1 {
		h, m = v.atHour, v.atMinute
		mode = "at"
	}
	hour, e1 := strconv.Atoi(h)
	minute, e2 := strconv.Atoi(m)
	if e1 != nil || e2 != nil || hour < 0 || minute < 0 || minute > 59 {
		return domain.Spec{}, fmt.Errorf("请输入有效时间")
	}
	return domain.Spec{Mode: mode, Minutes: hour*60 + minute, Hour: hour, Minute: minute}, nil
}

func (v *View) Render(c *ui.Context) {
	m := v.Backend.Snapshot()
	p := lightTheme(c)
	ui.Column(c).Fill().Padding(22, 26, 26).Gap(20).Children(func() {
		ui.Scroll(c.Key("page")).Grow(1).Children(func() {
			ui.Column(c.Key("card")).MinHeight(490).FillWidth().Padding(24, 24, 26).Gap(20).Radius(24).Background(p.surface).Border(1, p.line).Children(func() {
				if m.Notice != "" {
					ui.Row(c).Children(func() {
						ui.Spacer(c)
						if iconButton(c, "notice", "warning", "查看提示", p) {
							v.details = true
						}
					})
				}
				switch m.Timer.Phase {
				case domain.Running, domain.Grace, domain.Requesting:
					v.active(c, m, p)
				case domain.Completed, domain.Cancelled, domain.Failed:
					v.result(c, m, p)
				default:
					v.setup(c, m, p)
				}
			})
		})
	})
	v.detailsDialog(c, m, p)
}

func (v *View) setup(c *ui.Context, m application.Model, p palette) {
	seg := ui.SegmentedBase(c.Key("mode"), &v.mode, 2)
	seg.Track.Height(42).Padding(4).Gap(3).Radius(12).Background(p.soft).Children(func() {
		for i, label := range []string{"倒计时", "指定时间"} {
			item := seg.Segment(i).Grow(1).Height(34).Center().Radius(9)
			if v.mode == i {
				item.Background(p.surface).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.06))
			}
			item.Children(func() { ui.Text(c, label).FontSize(13).TextColor(p.text) })
		}
	})
	ui.Box(c).Size(58, 58).Margin(8, 0, 0).AlignSelf(ui.Center).Radius(29).Background(p.tint).Center().Children(func() { icon(c, "moon", 30, p.accent) })
	hours, minutes := &v.hours, &v.minutes
	if v.mode == 1 {
		hours, minutes = &v.atHour, &v.atMinute
	}
	ui.Row(c).Gap(4).Justify(ui.Center).AlignItems(ui.Start).Children(func() {
		v.timeField(c, "hours", hours, "小时", p)
		ui.Text(c, ":").FontSize(57).TextColor(p.line).Padding(5, 0, 0)
		v.timeField(c, "minutes", minutes, "分钟", p)
	})
	if v.mode == 0 {
		ui.Row(c).Gap(6).Children(func() {
			for _, preset := range []struct {
				minutes int
				label   string
			}{{30, "30分"}, {60, "1小时"}, {120, "2小时"}, {240, "4小时"}, {480, "8小时"}} {
				b := ui.ButtonBase(c.Key(preset.minutes)).Grow(1).Height(33).Center().Radius(9).Label(preset.label)
				color := p.muted
				if s, err := v.spec(); err == nil && s.Minutes == preset.minutes {
					b.Background(p.tint)
					color = p.accent
				} else if b.Hovered() {
					b.Background(p.soft)
				}
				b.Children(func() { ui.Text(c, preset.label).FontSize(12).TextColor(color).NoWrap() })
				if b.Clicked() {
					v.hours = fmt.Sprintf("%02d", preset.minutes/60)
					v.minutes = fmt.Sprintf("%02d", preset.minutes%60)
				}
			}
		})
	} else {
		ui.Box(c).Height(33)
	}
	ui.Spacer(c).MinHeight(0)
	spec, err := v.spec()
	target := time.Time{}
	if err == nil {
		target, err = spec.Trigger(m.Now)
	}
	v.timeCaption(c, m.Now, target.Add(domain.GracePeriod), err, p)
	ui.Row(c).FillWidth().Children(func() {
		if button(c, "start", "play", "开始计时", p, true, err != nil) {
			v.Backend.Start(spec)
		}
	})
}

func (v *View) timeField(c *ui.Context, key string, value *string, label string, p palette) {
	ui.Column(c.Key(key)).Width(112).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.TextInputBase(c.Key("input"), value).Width(112).Height(82).FontSize(66).FontWeight(500).FontFeatures("tnum").TextColor(p.text).TextAlign(ui.Center).Label(label).Placeholder("00")
		ui.Text(c, label).FontSize(11).TextColor(p.muted)
	})
}

func (v *View) timeCaption(c *ui.Context, now, target time.Time, err error, p palette) {
	ui.Row(c).Gap(7).Justify(ui.Center).Children(func() {
		icon(c, "moon", 14, p.muted)
		label := "— —"
		color := p.muted
		if err == nil {
			label = dayTime(now, target) + " 睡眠"
		} else {
			label, color = err.Error(), p.danger
		}
		ui.Text(c, label).FontSize(12).TextColor(color)
	})
}

func (v *View) active(c *ui.Context, m application.Model, p palette) {
	status := "计时中"
	warning := m.Timer.Phase == domain.Grace
	if warning {
		status = "即将睡眠"
	} else if m.Timer.Phase == domain.Requesting {
		status = "正在睡眠"
	}
	ui.Text(c, status).FontSize(14).TextColor(p.muted).AlignSelf(ui.Center).Margin(7, 0, 0)
	remaining := m.Timer.Remaining(m.Now)
	total := m.Timer.TriggerAt.Sub(m.Timer.StartedAt)
	if warning {
		total = domain.GracePeriod
	}
	progress := float64(remaining) / float64(max(time.Second, total))
	ring(c, p, progress, warning, func() {
		ui.Column(c).Fill().Center().Gap(12).Children(func() {
			color := p.accent
			if warning {
				color = p.danger
			}
			icon(c, "moon", 30, color)
			if m.Timer.Phase == domain.Requesting {
				ui.Spinner(c).Size(34, 34)
			} else {
				text := formatRemaining(remaining)
				if warning {
					text = fmt.Sprintf("%02d", int(math.Ceil(remaining.Seconds())))
				}
				ui.Text(c, text).FontSize(45).FontFeatures("tnum").FontWeight(500).TextColor(p.text)
			}
		})
	})
	v.timeCaption(c, m.Now, m.Timer.SleepAt, nil, p)
	ui.Spacer(c).MinHeight(0)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		disabled := !m.Timer.Cancellable()
		if button(c, "cancel", "close", "取消", p, false, disabled) {
			v.Backend.Cancel()
		}
		if button(c, "postpone", "plus", "延后15分钟", p, true, disabled) {
			v.Backend.Postpone()
		}
		if v.Hide != nil && iconButton(c, "hide", "tray", "收进托盘", p) {
			v.Hide()
		}
	})
}

func (v *View) result(c *ui.Context, m application.Model, p palette) {
	name, label, color := "check", "已结束", p.success
	if m.Timer.Phase == domain.Cancelled {
		name, label, color = "moon", "已取消", p.muted
	}
	if m.Timer.Phase == domain.Failed {
		name, label, color = "warning", "未能睡眠", p.danger
	}
	ui.Spacer(c)
	ui.Box(c).Size(88, 88).Radius(44).Background(p.soft).AlignSelf(ui.Center).Center().Children(func() { icon(c, name, 39, color) })
	ui.Text(c, label).FontSize(24).Bold().TextColor(p.text).AlignSelf(ui.Center)
	if m.Timer.Phase == domain.Failed {
		if ui.Button(c.Key("error-details"), "详情").AlignSelf(ui.Center).Clicked() {
			v.details = true
		}
	}
	ui.Spacer(c)
	ui.Row(c).Gap(10).Children(func() {
		if button(c, "reset", "reset", "重新设置", p, m.Timer.Phase != domain.Failed, false) {
			v.Backend.Reset()
		}
		if m.Timer.Phase == domain.Failed {
			if button(c, "retry", "play", "重试", p, true, false) {
				v.Backend.Retry()
			}
		}
	})
}

func (v *View) detailsDialog(c *ui.Context, m application.Model, p palette) {
	ui.DialogBase(c.Key("details-modal"), &v.details, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.30))
		panel.Width(400).MaxWidthPercent(90).Padding(24).Gap(18).Radius(20).Background(p.surface)
		ui.Row(c).Children(func() {
			ui.Text(c, "详情").FontSize(18).Bold().Grow(1)
			if iconButton(c, "close", "close", "关闭详情", p) {
				v.details = false
				v.Backend.Dismiss()
			}
		})
		ui.Scroll(c).MaxHeight(220).Gap(10).Children(func() {
			if m.Notice != "" {
				ui.Text(c, m.Notice).FontSize(13).TextColor(p.muted).Selectable()
			}
			if m.Timer.Detail != "" {
				ui.Text(c, m.Timer.Detail).FontSize(13).TextColor(p.muted).Selectable()
			}
		})
	})
}

func formatRemaining(d time.Duration) string {
	n := int(math.Ceil(d.Seconds()))
	return fmt.Sprintf("%02d:%02d:%02d", n/3600, n/60%60, n%60)
}
func dayTime(now, target time.Time) string {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	prefix := target.Format("1月2日")
	if day.Equal(today) {
		prefix = "今天"
	} else if day.Equal(today.AddDate(0, 0, 1)) {
		prefix = "明天"
	}
	return prefix + " " + target.Format("15:04")
}
