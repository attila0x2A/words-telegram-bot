// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	dir, err := ioutil.TempDir("", "load_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "tmpdb")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	count := func(table string) int32 {
		t.Helper()
		r := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s;", table))
		var n int32
		if err := r.Scan(&n); err != nil {
			t.Fatalf("count(%s): %v", table, err)
		}
		return n
	}

	l, err := NewLoader(dbPath, "../testdata/sentences.csv", "../testdata/links.csv")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	tables := []string{"Sentences", "Translations", "Words"}
	got := make(map[string]int32)
	for _, tb := range tables {
		n := count(tb)
		if n == 0 {
			t.Errorf("%s: got %d want > 0", tb, n)
		}
		got[tb] = n
	}

	// Concecutive calls to Load should result in no errors and be noops.
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	want := got
	for _, tb := range tables {
		got[tb] = count(tb)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	log.Printf("want: %v", want)
}
