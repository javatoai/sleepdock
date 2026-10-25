package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sleepdock/internal/domain"
	"sync"
	"time"
)

type Preferences struct {
	Path    string
	mu      sync.Mutex
	invalid bool
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "SleepDock", "settings.json"), nil
}
func (s *Preferences) Load() (domain.Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := domain.Defaults()
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(data) > 1<<20 || !json.Valid(data) {
		s.invalid = true
		return p, fmt.Errorf("设置文件损坏，保存时会备份原文件")
	}
	var saved domain.Preferences
	if err = json.Unmarshal(data, &saved); err != nil {
		s.invalid = true
		return p, err
	}
	if saved.Version != 1 {
		return p, fmt.Errorf("设置版本不匹配：%d", saved.Version)
	}
	if _, err = saved.Last.Trigger(time.Now()); err != nil {
		saved.Last = domain.DefaultSpec()
	}
	return saved, nil
}
func (s *Preferences) Save(p domain.Preferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	if s.invalid {
		if err := os.Rename(s.Path, s.Path+".corrupt-"+time.Now().Format("20060102-150405.000000000")); err != nil {
			return err
		}
		s.invalid = false
	}
	if data, err := os.ReadFile(s.Path); err == nil {
		var header struct {
			Version int `json:"version"`
		}
		if json.Unmarshal(data, &header) == nil && header.Version != 1 {
			return fmt.Errorf("设置版本不匹配，原文件已保留")
		}
	}
	p.Version = 1
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.Path)
}
