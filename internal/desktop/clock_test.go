package desktop

import (
	"fmt"
	"sleepdock/internal/domain"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCountdownKeepsEverySecondWithSchedulingJitter(t *testing.T) {
	for _, state := range []string{"running", "grace"} {
		t.Run(state, func(t *testing.T) {
			v := Preview(state)
			b := v.Backend.(*previewBackend)
			tt := ui.NewTester(v.Render, 560, 590)
			trigger, sleep := b.model.Timer.TriggerAt, b.model.Timer.SleepAt
			initial := int(b.model.Timer.Remaining(b.model.Now) / time.Second)
			// 交替模拟主线程和系统计时器的延迟，不能依赖一次 tick 恰好是一秒。
			jitter := []time.Duration{2 * time.Millisecond, 150 * time.Millisecond, 4 * time.Millisecond, 80 * time.Millisecond}
			for i := 1; i <= min(30, initial-1); i++ {
				b.model.Now = b.model.Now.Add(nextTickDelay(b.model) + jitter[(i-1)%len(jitter)])
				b.model.Timer.Tick(b.model.Now)
				tt.Frame()
				n := initial - i
				want := fmt.Sprintf("%02d:%02d:%02d", n/3600, n/60%60, n%60)
				if state == "grace" {
					want = fmt.Sprintf("%02d", n)
				}
				if _, ok := tt.Find(want); !ok {
					t.Fatalf("refresh %d skipped a second: want %q, got %v", i, want, tt.Texts())
				}
			}
			if !b.model.Timer.TriggerAt.Equal(trigger) || !b.model.Timer.SleepAt.Equal(sleep) {
				t.Fatal("display refresh changed the actual deadlines")
			}
		})
	}
}

func TestCountdownRefreshAtDeadlineAndAfterPostpone(t *testing.T) {
	v := Preview("running")
	b := v.Backend.(*previewBackend)
	b.model.Now = b.model.Timer.TriggerAt
	if delay := nextTickDelay(b.model); delay <= 0 || delay > time.Millisecond {
		t.Fatalf("due timer must be processed promptly, got %v", delay)
	}
	remind, sleep := b.model.Timer.Tick(b.model.Now)
	if !remind || sleep || b.model.Timer.Phase != domain.Grace {
		t.Fatal("refresh skipped the reminder")
	}
	if delay := nextTickDelay(b.model); delay != time.Second {
		t.Fatalf("new reminder should show its first second in full, got %v", delay)
	}
	b.model.Now = b.model.Now.Add(123 * time.Millisecond)
	b.Postpone()
	if delay := nextTickDelay(b.model); delay != time.Second {
		t.Fatalf("refresh did not realign to the postponed deadline, got %v", delay)
	}
	b.Cancel()
	if delay := nextTickDelay(b.model); delay != time.Second {
		t.Fatalf("inactive timer should not busy-loop, got %v", delay)
	}
}
