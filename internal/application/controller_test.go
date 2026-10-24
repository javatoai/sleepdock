package application

import (
	"context"
	"errors"
	"sleepdock/internal/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fakeStore struct {
	prefs domain.Preferences
	fail  bool
}

func (s *fakeStore) Load() (domain.Preferences, error) { return s.prefs, nil }
func (s *fakeStore) Save(p domain.Preferences) error {
	if s.fail {
		return errors.New("disk full")
	}
	s.prefs = p
	return nil
}

type sleepFunc func(context.Context) error

func (f sleepFunc) Sleep(ctx context.Context) error { return f(ctx) }

type notifyFunc func() error

func (f notifyFunc) Remind() error { return f() }

func fixture(t *testing.T, power Sleeper, notifier Notifier, store *fakeStore) (*Controller, *fakeClock, chan struct{}) {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 10, 24, 22, 0, 0, 0, time.UTC)}
	events := make(chan struct{}, 100)
	c := New(clock, power, notifier, store, func() {
		select {
		case events <- struct{}{}:
		default:
		}
	})
	t.Cleanup(c.Close)
	return c, clock, events
}
func waitPhase(t *testing.T, c *Controller, events chan struct{}, phase domain.Phase) {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		if c.Snapshot().Timer.Phase == phase {
			return
		}
		select {
		case <-events:
		case <-timeout.C:
			t.Fatal("phase:", c.Snapshot().Timer.Phase, "wanted", phase)
		}
	}
}
func TestControllerEffectAtMostOnceUnderConcurrentTicks(t *testing.T) {
	var sleeps, reminders atomic.Int32
	c, clock, events := fixture(t, sleepFunc(func(context.Context) error { sleeps.Add(1); return nil }), notifyFunc(func() error { reminders.Add(1); return nil }), &fakeStore{prefs: domain.Defaults()})
	c.Start(domain.Spec{Mode: "duration", Minutes: 30})
	clock.Advance(30 * time.Minute)
	c.Tick()
	if reminders.Load() != 1 {
		t.Fatal(reminders.Load())
	}
	clock.Advance(time.Minute)
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() { defer wg.Done(); c.Tick() }()
	}
	wg.Wait()
	waitPhase(t, c, events, domain.Completed)
	if sleeps.Load() != 1 || reminders.Load() != 1 {
		t.Fatal(sleeps.Load(), reminders.Load())
	}
}
func TestControllerCancellationAndEarlySystemSleep(t *testing.T) {
	for _, cancel := range []bool{true, false} {
		var calls atomic.Int32
		c, clock, _ := fixture(t, sleepFunc(func(context.Context) error { calls.Add(1); return nil }), notifyFunc(func() error { return nil }), &fakeStore{prefs: domain.Defaults()})
		c.Start(domain.DefaultSpec())
		if cancel {
			c.Cancel()
		} else {
			c.SystemSlept()
		}
		clock.Advance(12 * time.Hour)
		c.Tick()
		c.SystemSlept()
		if calls.Load() != 0 || c.Snapshot().Timer.Active() {
			t.Fatal("inactive timer fired")
		}
	}
}
func TestControllerFailureRetryAndClose(t *testing.T) {
	var calls atomic.Int32
	c, clock, events := fixture(t, sleepFunc(func(context.Context) error { calls.Add(1); return errors.New("permission denied") }), notifyFunc(func() error { return nil }), &fakeStore{prefs: domain.Defaults()})
	c.Start(domain.Spec{Mode: "duration", Minutes: 1})
	clock.Advance(time.Minute)
	c.Tick()
	clock.Advance(time.Minute)
	c.Tick()
	waitPhase(t, c, events, domain.Failed)
	c.Retry()
	if c.Snapshot().Timer.Phase != domain.Grace {
		t.Fatal("missing retry reminder")
	}
	c.Close()
	clock.Advance(time.Minute)
	c.Tick()
	if calls.Load() != 1 {
		t.Fatal("closed controller fired")
	}
}
func TestSettingsRestoreDoesNotRestoreActivePlan(t *testing.T) {
	store := &fakeStore{prefs: domain.Defaults()}
	store.prefs.Last = domain.Spec{Mode: "at", Hour: 2, Minute: 30}
	c, _, _ := fixture(t, sleepFunc(func(context.Context) error { t.Fatal("unexpected sleep"); return nil }), notifyFunc(func() error { return nil }), store)
	if c.Snapshot().Timer.Phase != domain.Idle || c.Snapshot().Preferences.Last.Hour != 2 {
		t.Fatal(c.Snapshot())
	}
	c.SetTheme("dark")
	if c.Snapshot().Preferences.Theme != "dark" {
		t.Fatal("theme not saved")
	}
}
func TestFailedNotificationDoesNotSkipGrace(t *testing.T) {
	c, clock, _ := fixture(t, sleepFunc(func(context.Context) error { return nil }), notifyFunc(func() error { return errors.New("not supported") }), &fakeStore{prefs: domain.Defaults(), fail: true})
	c.Start(domain.Spec{Mode: "duration", Minutes: 1})
	if c.Snapshot().Notice == "" {
		t.Fatal("save failure lost")
	}
	clock.Advance(time.Minute)
	c.Tick()
	if c.Snapshot().Timer.Phase != domain.Grace || c.Snapshot().Notice == "" {
		t.Fatal(c.Snapshot())
	}
}
