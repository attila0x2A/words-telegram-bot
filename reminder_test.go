package main

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestReminders(t *testing.T) {
	dir, err := ioutil.TempDir("", "repetition")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Temp dir: %q", dir)
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "tmpdb")

	settings, err := NewSettingsConfig(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReminder(&Clients{
		Settings: settings,
	}, db)
	if err != nil {
		t.Fatal(err)
	}

	var chatID int64 = 0
	s := DefaultSettings()
	var startSeconds int64 = 10 * 60 * 60
	var endSeconds int64 = 19 * 60 * 60
	s.AvailibilityWindows = []AvailabilityWindow{
		{
			StartSeconds: startSeconds,
			EndSeconds:   endSeconds,
		},
	}
	if err := settings.Set(chatID, s); err != nil {
		t.Fatal(err)
	}

	c := make(chan time.Time)

	cancel := make(chan struct{})
	go func() {
		c <- time.Now()
		cancel <- struct{}{}
		cancel <- struct{}{}
	}()

	var sent []*Notification
	r.sendNofication = func(n *Notification) error {
		sent = append(sent, n)
		return nil
	}

	timeNow = func() time.Time {
		return time.Date(2020, 1, 1, 0, 0, int(startSeconds)+1, 0, time.UTC)
	}

	r.Loop(c, cancel)

	if len(sent) != 1 {
		t.Errorf("got %d notifications (%v), want 1", len(sent), sent)
	}

	chatID = 1
	if err := settings.Set(chatID, s); err != nil {
		t.Fatal(err)
	}

	timeNow = func() time.Time {
		return time.Date(2020, 1, 1, 0, 0, int(endSeconds)+1, 0, time.UTC)
	}

	r.Loop(c, cancel)

	if len(sent) != 1 {
		t.Errorf("got %d notifications (%v), want 1", len(sent), sent)
	}
}
