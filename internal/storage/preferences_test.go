package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sleepdock/internal/domain"
	"testing"
)

func TestPreferencesRoundtripAndNoActivePlan(t *testing.T) {
	s := &Preferences{Path: filepath.Join(t.TempDir(), "settings.json")}
	p, err := s.Load()
	if err != nil || p.Theme != "system" {
		t.Fatal(p, err)
	}
	p.Theme = "dark"
	p.Last = domain.Spec{Mode: "at", Hour: 3, Minute: 15}
	for range 2 {
		if err := s.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load()
	if err != nil || got.Last != p.Last || got.Theme != "dark" {
		t.Fatal(got, err)
	}
	data, _ := os.ReadFile(s.Path)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 {
		t.Fatal("unexpected persisted state", fields)
	}
}
func TestCorruptBackupAndFutureSchemaProtection(t *testing.T) {
	for _, raw := range []string{"broken", `{"version":9,"future":true}`} {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		s := &Preferences{Path: path}
		if _, err := s.Load(); err == nil {
			t.Fatal("invalid file accepted")
		}
		err := s.Save(domain.Defaults())
		if raw == "broken" {
			if err != nil {
				t.Fatal(err)
			}
			files, _ := filepath.Glob(path + ".corrupt-*")
			if len(files) != 1 {
				t.Fatal(files)
			}
		} else {
			if err == nil {
				t.Fatal("future schema overwritten")
			}
			data, _ := os.ReadFile(path)
			if string(data) != raw {
				t.Fatal(string(data))
			}
		}
	}
}
