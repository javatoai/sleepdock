package desktop

import (
	"sleepdock/internal/application"
	"sleepdock/internal/domain"
	"time"
)

// Preview uses an offline in-memory backend. It cannot request system sleep.
func Preview(state string) *View {
	now := time.Date(2026, 10, 24, 22, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	m := application.Model{Preferences: domain.Defaults(), Timer: domain.Timer{Phase: domain.Idle}, Now: now}
	if state == "running" || state == "grace" {
		_ = m.Timer.Start(now.Add(-25*time.Minute), domain.Spec{Mode: "duration", Minutes: 120})
		if state == "grace" {
			m.Timer.TriggerAt = now
			m.Timer.Tick(now)
			m.Timer.SleepAt = now.Add(42 * time.Second)
		}
	}
	if state == "completed" {
		m.Timer.Phase = domain.Completed
	}
	if state == "error" {
		m.Timer.Phase = domain.Failed
		m.Timer.Detail = "系统未接受睡眠请求。请检查本机电源权限。"
	}
	v := NewView(&previewBackend{model: m})
	if state == "at" {
		v.mode = 1
		v.atHour = "02"
		v.atMinute = "00"
	}
	return v
}

type previewBackend struct {
	model                            application.Model
	starts, postpones, cancels, nows int
}

func (b *previewBackend) Snapshot() application.Model { return b.model }
func (b *previewBackend) Start(s domain.Spec) {
	b.starts++
	if err := b.model.Timer.Start(b.model.Now, s); err != nil {
		b.model.Notice = err.Error()
	}
}
func (b *previewBackend) SleepNow() { b.nows++; b.model.Timer.SleepNow(b.model.Now) }
func (b *previewBackend) Postpone() { b.postpones++; b.model.Timer.Postpone(b.model.Now) }
func (b *previewBackend) Cancel()   { b.cancels++; b.model.Timer.Cancel() }
func (b *previewBackend) Retry()    { b.model.Timer.Retry(b.model.Now) }
func (b *previewBackend) Reset()    { b.model.Timer.Phase = domain.Idle }
func (b *previewBackend) Dismiss()  { b.model.Notice = "" }
