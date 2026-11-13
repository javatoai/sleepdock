package domain

import (
	"testing"
	"time"
)

func testNow() time.Time {
	return time.Date(2026, 10, 24, 23, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
}
func TestTimeSelectionAndMidnight(t *testing.T) {
	now := testNow()
	for _, tc := range []struct {
		spec     Spec
		expected time.Time
	}{
		{Spec{Mode: "duration", Minutes: 30}, now.Add(30 * time.Minute)},
		{Spec{Mode: "duration", Minutes: 480}, now.Add(8 * time.Hour)},
		{Spec{Mode: "at", Hour: 2}, time.Date(2026, 10, 25, 2, 0, 0, 0, now.Location())},
		{Spec{Mode: "at", Hour: 23, Minute: 45}, now.Add(15 * time.Minute)},
		{Spec{Mode: "at", Hour: 23, Minute: 30}, now.AddDate(0, 0, 1)},
	} {
		target, err := tc.spec.Trigger(now)
		if err != nil || !target.Equal(tc.expected) {
			t.Fatal(tc.spec, target, err)
		}
	}
	for _, s := range []Spec{{Mode: "bad"}, {Mode: "duration", Minutes: 0}, {Mode: "duration", Minutes: 1441}, {Mode: "at", Hour: 24}, {Mode: "at", Minute: 60}} {
		if _, err := s.Trigger(now); err == nil {
			t.Fatal("accepted invalid input", s)
		}
	}
}

func TestReminderAndSleepAreOneShot(t *testing.T) {
	now := testNow()
	var timer Timer
	if err := timer.Start(now, Spec{Mode: "duration", Minutes: 30}); err != nil {
		t.Fatal(err)
	}
	if !timer.SleepAt.Equal(now.Add(31 * time.Minute)) {
		t.Fatal("missing reminder minute")
	}
	if r, s := timer.Tick(now.Add(29 * time.Minute)); r || s {
		t.Fatal("fired early")
	}
	if r, s := timer.Tick(now.Add(30 * time.Minute)); !r || s {
		t.Fatal("missing reminder")
	}
	if r, s := timer.Tick(now.Add(30*time.Minute + 59*time.Second)); r || s {
		t.Fatal("slept early")
	}
	if r, s := timer.Tick(now.Add(31 * time.Minute)); r || !s {
		t.Fatal("missing sleep")
	}
	for range 5 {
		if r, s := timer.Tick(now.Add(2 * time.Hour)); r || s {
			t.Fatal("duplicate effect")
		}
	}
	id := timer.ID
	timer.Finish(id, nil)
	if timer.Phase != Completed {
		t.Fatal(timer.Phase)
	}
}

func TestLateTickStillGivesFullMinute(t *testing.T) {
	now := testNow()
	var timer Timer
	_ = timer.Start(now, Spec{Mode: "duration", Minutes: 30})
	late := now.Add(3 * time.Hour)
	if r, s := timer.Tick(late); !r || s || !timer.SleepAt.Equal(late.Add(time.Minute)) {
		t.Fatal(timer, r, s)
	}
}

func TestSleepNowStartsFullReminderOnce(t *testing.T) {
	now := testNow()
	timer := Timer{ID: 3, Phase: Running, StartedAt: now, TriggerAt: now.Add(time.Hour), SleepAt: now.Add(time.Hour + time.Minute)}
	if !timer.SleepNow(now) {
		t.Fatal("running timer rejected immediate sleep")
	}
	if timer.ID != 4 || timer.Phase != Grace || !timer.TriggerAt.Equal(now) || !timer.SleepAt.Equal(now.Add(time.Minute)) {
		t.Fatal(timer)
	}
	if timer.SleepNow(now.Add(time.Second)) {
		t.Fatal("restarted an existing reminder")
	}
	if r, s := timer.Tick(now.Add(59 * time.Second)); r || s {
		t.Fatal("slept before the reminder minute")
	}
	if r, s := timer.Tick(now.Add(time.Minute)); r || !s || timer.Phase != Requesting {
		t.Fatal(timer, r, s)
	}
	if timer.SleepNow(now.Add(time.Minute)) {
		t.Fatal("duplicated an in-flight sleep request")
	}
	if timer.Phase != Requesting {
		t.Fatal(timer.Phase)
	}
}

func TestSleepNowFromInactiveAndFailedStates(t *testing.T) {
	for _, phase := range []Phase{Idle, Completed, Cancelled, Failed} {
		now := testNow()
		timer := Timer{ID: 5, Phase: phase, Detail: "old"}
		if !timer.SleepNow(now) || timer.Phase != Grace || !timer.SleepAt.Equal(now.Add(time.Minute)) || timer.Detail != "" {
			t.Fatalf("%s: %+v", phase, timer)
		}
		if r, s := timer.Tick(now.Add(time.Minute)); r || !s {
			t.Fatalf("%s: missing sleep effect", phase)
		}
	}
}

func TestPostponeWhileRunningAndDuringReminder(t *testing.T) {
	now := testNow()
	var timer Timer
	_ = timer.Start(now, Spec{Mode: "duration", Minutes: 30})
	timer.Postpone(now.Add(5 * time.Minute))
	if !timer.TriggerAt.Equal(now.Add(45 * time.Minute)) {
		t.Fatal(timer)
	}
	timer.Tick(now.Add(45 * time.Minute))
	timer.Postpone(now.Add(45*time.Minute + 30*time.Second))
	if timer.Phase != Running || !timer.TriggerAt.Equal(now.Add(60*time.Minute+30*time.Second)) {
		t.Fatal(timer)
	}
}

func TestCancelAndSystemSleepConsumePlan(t *testing.T) {
	for _, phase := range []Phase{Running, Grace, Requesting} {
		now := testNow()
		timer := Timer{ID: 4, Phase: phase, TriggerAt: now, SleepAt: now}
		timer.SystemSlept()
		timer.SystemSlept()
		if timer.Phase != Completed {
			t.Fatal(timer)
		}
		if r, s := timer.Tick(now.AddDate(0, 0, 1)); r || s {
			t.Fatal("resumed into another sleep")
		}
	}
	var timer Timer
	_ = timer.Start(testNow(), DefaultSpec())
	timer.Cancel()
	if timer.Phase != Cancelled || timer.Active() {
		t.Fatal(timer)
	}
	if r, s := timer.Tick(testNow().Add(10 * time.Hour)); r || s {
		t.Fatal("cancelled plan fired")
	}
}

func TestFailureRetryAndStaleResults(t *testing.T) {
	now := testNow()
	timer := Timer{ID: 7, Phase: Failed, Detail: "denied"}
	if !timer.Retry(now) || timer.Phase != Grace || !timer.SleepAt.Equal(now.Add(time.Minute)) {
		t.Fatal(timer)
	}
	timer.Finish(7, nil)
	if timer.Phase != Grace {
		t.Fatal("old result changed retry")
	}
	timer.Tick(now.Add(time.Minute))
	timer.Finish(timer.ID, nil)
	if timer.Phase != Completed {
		t.Fatal(timer)
	}
}
