package domain

import (
	"fmt"
	"time"
)

const GracePeriod = time.Minute
const PostponePeriod = 15 * time.Minute

type Spec struct {
	Mode    string `json:"mode"`
	Minutes int    `json:"minutes"`
	Hour    int    `json:"hour"`
	Minute  int    `json:"minute"`
}

func DefaultSpec() Spec { return Spec{Mode: "duration", Minutes: 120, Hour: 2} }

func (s Spec) Trigger(now time.Time) (time.Time, error) {
	switch s.Mode {
	case "duration":
		if s.Minutes < 1 || s.Minutes > 24*60 {
			return time.Time{}, fmt.Errorf("请输入 1–1440 分钟")
		}
		return now.Add(time.Duration(s.Minutes) * time.Minute), nil
	case "at":
		if s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 {
			return time.Time{}, fmt.Errorf("请输入有效时间")
		}
		target := time.Date(now.Year(), now.Month(), now.Day(), s.Hour, s.Minute, 0, 0, now.Location())
		if !target.After(now) {
			target = time.Date(now.Year(), now.Month(), now.Day()+1, s.Hour, s.Minute, 0, 0, now.Location())
		}
		if target.Hour() != s.Hour || target.Minute() != s.Minute {
			return time.Time{}, fmt.Errorf("该日期没有此时间，请重新选择")
		}
		return target, nil
	default:
		return time.Time{}, fmt.Errorf("请选择计时方式")
	}
}

type Phase string

const (
	Idle       Phase = "idle"
	Running    Phase = "running"
	Grace      Phase = "grace"
	Requesting Phase = "requesting"
	Completed  Phase = "completed"
	Cancelled  Phase = "cancelled"
	Failed     Phase = "failed"
)

type Timer struct {
	ID        uint64
	Phase     Phase
	Spec      Spec
	TriggerAt time.Time
	SleepAt   time.Time
	StartedAt time.Time
	Detail    string
}

func (t Timer) Active() bool      { return t.Phase == Running || t.Phase == Grace || t.Phase == Requesting }
func (t Timer) Editable() bool    { return !t.Active() }
func (t Timer) Cancellable() bool { return t.Phase == Running || t.Phase == Grace }

func (t *Timer) Start(now time.Time, spec Spec) error {
	if t.Active() {
		return fmt.Errorf("已有计时正在运行")
	}
	trigger, err := spec.Trigger(now)
	if err != nil {
		return err
	}
	*t = Timer{ID: t.ID + 1, Phase: Running, Spec: spec, StartedAt: now, TriggerAt: trigger, SleepAt: trigger.Add(GracePeriod)}
	return nil
}

// Tick returns the two one-shot effects. A late tick always allows a full
// reminder minute, including after a wall-clock jump or a suspended event loop.
func (t *Timer) Tick(now time.Time) (remind, sleep bool) {
	if t.Phase == Running && !now.Before(t.TriggerAt) {
		t.Phase = Grace
		t.SleepAt = now.Add(GracePeriod)
		return true, false
	}
	if t.Phase == Grace && !now.Before(t.SleepAt) {
		t.Phase = Requesting
		return false, true
	}
	return false, false
}

func (t *Timer) Postpone(now time.Time) {
	if !t.Cancellable() {
		return
	}
	base := t.TriggerAt
	if now.After(base) {
		base = now
	}
	t.ID++
	t.Phase, t.TriggerAt = Running, base.Add(PostponePeriod)
	t.SleepAt, t.Detail = t.TriggerAt.Add(GracePeriod), ""
}

func (t *Timer) Cancel() {
	if t.Cancellable() {
		t.ID++
		t.Phase, t.Detail = Cancelled, ""
	}
}

func (t *Timer) SystemSlept() {
	if t.Active() {
		t.Phase, t.Detail = Completed, ""
	}
}

func (t *Timer) Finish(id uint64, err error) {
	if t.ID != id || t.Phase != Requesting {
		return
	}
	t.Phase, t.Detail = Completed, ""
	if err != nil {
		t.Phase, t.Detail = Failed, err.Error()
	}
}

func (t *Timer) Retry(now time.Time) bool {
	if t.Phase != Failed {
		return false
	}
	t.ID++
	t.Phase, t.StartedAt, t.TriggerAt, t.SleepAt, t.Detail = Grace, now, now, now.Add(GracePeriod), ""
	return true
}

func (t Timer) Remaining(now time.Time) time.Duration {
	end := t.TriggerAt
	if t.Phase == Grace {
		end = t.SleepAt
	}
	if t.Phase != Running && t.Phase != Grace {
		return 0
	}
	return max(0, end.Sub(now))
}

type Preferences struct {
	Version int  `json:"version"`
	Last    Spec `json:"last"`
}

func Defaults() Preferences { return Preferences{Version: 1, Last: DefaultSpec()} }
