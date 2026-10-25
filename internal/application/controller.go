package application

import (
	"context"
	"fmt"
	"sleepdock/internal/domain"
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type Sleeper interface{ Sleep(context.Context) error }
type Notifier interface{ Remind() error }
type Store interface {
	Load() (domain.Preferences, error)
	Save(domain.Preferences) error
}

type Model struct {
	Timer       domain.Timer
	Preferences domain.Preferences
	Now         time.Time
	Notice      string
}

type Controller struct {
	mu       sync.Mutex
	model    Model
	clock    Clock
	sleeper  Sleeper
	notifier Notifier
	store    Store
	changed  func()
	ctx      context.Context
	cancel   context.CancelFunc
	workers  sync.WaitGroup
	closed   bool
}

func New(clock Clock, sleeper Sleeper, notifier Notifier, store Store, changed func()) *Controller {
	prefs, err := store.Load()
	ctx, cancel := context.WithCancel(context.Background())
	c := &Controller{clock: clock, sleeper: sleeper, notifier: notifier, store: store, changed: changed, ctx: ctx, cancel: cancel}
	c.model = Model{Timer: domain.Timer{Phase: domain.Idle}, Preferences: prefs}
	if err != nil {
		c.model.Notice = err.Error()
	}
	return c
}

func (c *Controller) Snapshot() Model {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.model
	m.Now = c.clock.Now()
	return m
}
func (c *Controller) notify() {
	if c.changed != nil {
		c.changed()
	}
}

func (c *Controller) Start(spec domain.Spec) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if err := c.model.Timer.Start(c.clock.Now(), spec); err != nil {
		c.model.Notice = err.Error()
	} else {
		c.model.Notice = ""
		c.model.Preferences.Last = spec
		c.saveLocked()
	}
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) Tick() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	active := c.model.Timer.Active()
	remind, sleep := c.model.Timer.Tick(c.clock.Now())
	id := c.model.Timer.ID
	if sleep {
		c.workers.Add(1)
	}
	c.mu.Unlock()
	if active {
		c.notify()
	}
	if remind {
		c.remind()
	}
	if sleep {
		go func() {
			defer c.workers.Done()
			err := c.sleeper.Sleep(c.ctx)
			c.mu.Lock()
			c.model.Timer.Finish(id, err)
			c.mu.Unlock()
			c.notify()
		}()
	}
}

func (c *Controller) remind() {
	if err := c.notifier.Remind(); err != nil {
		c.Report(fmt.Errorf("系统通知未送达：%w", err))
	}
}

func (c *Controller) Postpone() {
	c.mu.Lock()
	if !c.closed {
		c.model.Timer.Postpone(c.clock.Now())
	}
	c.mu.Unlock()
	c.notify()
}
func (c *Controller) Cancel() { c.mu.Lock(); c.model.Timer.Cancel(); c.mu.Unlock(); c.notify() }
func (c *Controller) SystemSlept() {
	c.mu.Lock()
	c.model.Timer.SystemSlept()
	c.mu.Unlock()
	c.notify()
}
func (c *Controller) Reset() {
	c.mu.Lock()
	if c.model.Timer.Editable() {
		c.model.Timer.Phase = domain.Idle
		c.model.Timer.Detail = ""
	}
	c.mu.Unlock()
	c.notify()
}
func (c *Controller) Retry() {
	c.mu.Lock()
	ok := !c.closed && c.model.Timer.Retry(c.clock.Now())
	c.mu.Unlock()
	if ok {
		c.notify()
		c.remind()
	}
}
func (c *Controller) Dismiss() { c.mu.Lock(); c.model.Notice = ""; c.mu.Unlock(); c.notify() }
func (c *Controller) Report(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	c.model.Notice = err.Error()
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) saveLocked() {
	if err := c.store.Save(c.model.Preferences); err != nil {
		c.model.Notice = "设置未保存：" + err.Error()
	}
}
func (c *Controller) Close() {
	c.mu.Lock()
	c.closed = true
	c.model.Timer.Cancel()
	c.cancel()
	c.mu.Unlock()
	c.workers.Wait()
}
