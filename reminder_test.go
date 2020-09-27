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

	const chatID int64 = 0
	if err := settings.Set(chatID, DefaultSettings()); err != nil {
		t.Fatal(err)
	}

	c := make(chan time.Time)

	cancel := make(chan struct{})
	go func() {
		c <- time.Now()
		cancel <- struct{}{}
	}()

	var sent []*Notification
	r.sendNofication = func(n *Notification) error {
		sent = append(sent, n)
		return nil
	}

	r.Loop(c, cancel)

	if len(sent) != 1 {
		t.Errorf("got %d notifications (%v), want 1", len(sent), sent)
	}
}
