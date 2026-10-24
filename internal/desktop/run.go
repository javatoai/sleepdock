package desktop

import (
	"context"
	"fmt"
	"runtime"
	"sleepdock/assets"
	"sleepdock/internal/application"
	"sleepdock/internal/domain"
	"sleepdock/internal/platform"
	"sleepdock/internal/storage"
	"time"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
)

type nativeApp struct {
	controller                                     *application.Controller
	window                                         *mygo.Window
	tray                                           *mygo.Tray
	notification                                   *mygo.Notification
	statusItem, postponeItem, cancelItem, quitItem *mygo.MenuItem
	quitting, allowQuit                            bool
	stop                                           context.CancelFunc
}

func Run(configPath string) error {
	mygo.App.SetName("SleepDock")
	mygo.App.SetLanguages("zh-Hans")
	mygo.App.WhenReady(func() {
		s := &nativeApp{}
		s.controller = application.New(application.SystemClock{}, platform.Power{}, s, &storage.Preferences{Path: configPath}, func() {
			if s.window != nil {
				s.window.Update(s.changed)
			}
		})
		view := NewView(s.controller)
		view.Hide = func() {
			if s.tray != nil {
				s.window.Hide()
			}
		}
		s.window = mygo.NewWindow(mygo.WindowOptions{Title: "SleepDock", Width: 560, Height: 640, MinWidth: 480, MinHeight: 600, Content: ui.View(view.Render)})
		if err := s.window.SetIcon(assets.Icon); err != nil {
			s.controller.Report(err)
		}
		s.createTray()
		s.window.OnClose(func(e *mygo.CloseEvent) {
			if s.allowQuit {
				return
			}
			e.PreventDefault()
			if s.controller.Snapshot().Timer.Active() && s.tray != nil && !s.quitting {
				s.window.Hide()
			} else {
				s.quit()
			}
		})
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if !s.allowQuit {
				e.PreventDefault()
				s.quit()
			}
		})
		mygo.Power.OnSuspend(s.controller.SystemSlept)
		mygo.Power.OnResume(s.controller.SystemSlept)
		mygo.App.OnNotificationClick(func(id string) {
			if id == "sleepdock-reminder" {
				s.show()
			}
		})
		ctx, cancel := context.WithCancel(context.Background())
		s.stop = cancel
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.controller.Tick()
				}
			}
		}()
		s.changed()
	})
	return mygo.App.Run()
}

func (s *nativeApp) show() { s.window.Show(); s.window.Focus() }
func (s *nativeApp) createTray() {
	s.statusItem = &mygo.MenuItem{Label: "SleepDock", Disabled: true}
	s.postponeItem = &mygo.MenuItem{Label: "延后15分钟", Click: func(*mygo.MenuItem, *mygo.Window) { s.controller.Postpone() }}
	s.cancelItem = &mygo.MenuItem{Label: "取消计划", Click: func(*mygo.MenuItem, *mygo.Window) { s.controller.Cancel() }}
	s.quitItem = &mygo.MenuItem{Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) { s.quit() }}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		s.statusItem, mygo.Separator(), {Label: "打开 SleepDock", Click: func(*mygo.MenuItem, *mygo.Window) { s.show() }},
		s.postponeItem, s.cancelItem, mygo.Separator(), s.quitItem,
	})
	trayIcon := assets.TrayColor
	if runtime.GOOS == "darwin" {
		trayIcon = assets.Tray
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon, IconIsTemplate: runtime.GOOS == "darwin", ToolTip: "SleepDock", Menu: menu})
	if err != nil {
		s.controller.Report(fmt.Errorf("托盘不可用，关闭窗口将退出：%w", err))
		return
	}
	s.tray = tray
}

func (s *nativeApp) changed() {
	m := s.controller.Snapshot()
	if s.tray != nil {
		label := "未设定"
		if m.Timer.Phase == domain.Running || m.Timer.Phase == domain.Grace {
			label = formatRemaining(m.Timer.Remaining(m.Now))
		}
		if m.Timer.Phase == domain.Requesting {
			label = "正在睡眠"
		}
		s.statusItem.SetLabel(label)
		s.postponeItem.SetEnabled(m.Timer.Cancellable())
		s.cancelItem.SetEnabled(m.Timer.Cancellable())
		quit := "退出"
		if m.Timer.Active() {
			quit = "退出并取消计划"
		}
		s.quitItem.SetLabel(quit)
		s.tray.SetToolTip("SleepDock · " + label)
	}
	if m.Timer.Phase != domain.Grace && s.notification != nil {
		s.notification.Close()
		s.notification = nil
	}
}

func (s *nativeApp) Remind() error {
	s.window.Update(func() {
		if s.quitting || s.controller.Snapshot().Timer.Phase != domain.Grace {
			return
		}
		s.show()
		s.notification = mygo.NewNotification(mygo.NotificationOptions{ID: "sleepdock-reminder", Title: "SleepDock", Body: "60秒后睡眠"})
		s.notification.OnClick(s.show)
		if err := s.notification.Show(); err != nil {
			s.controller.Report(err)
		}
	})
	return nil
}

func (s *nativeApp) quit() {
	if s.quitting {
		return
	}
	s.quitting = true
	if s.stop != nil {
		s.stop()
	}
	s.controller.Cancel()
	go func() {
		s.controller.Close()
		s.window.Update(func() {
			s.allowQuit = true
			if s.notification != nil {
				s.notification.Close()
			}
			if s.tray != nil {
				s.tray.Destroy()
			}
			mygo.App.Quit()
		})
	}()
}
