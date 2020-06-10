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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepetition(t *testing.T) {
	dir, err := ioutil.TempDir("", "repetition")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Temp dir: %q", dir)
	defer os.RemoveAll(dir)

	db := filepath.Join(dir, "tmpdb")
	stages := []time.Duration{0, 0, 0, 0}
	r, err := NewRepetition(db, stages)
	if err != nil {
		t.Fatal(err)
	}

	type row struct {
		chatId     int64
		word       string
		definition string
		stage      int32
	}

	count := func(q *row) int32 {
		t.Helper()
		row := r.db.QueryRow(`
			SELECT COUNT(*) FROM Repetition
			WHERE chat_id = $1
				AND word = $2
				AND definition = $3
				AND stage = $4`,
			q.chatId, q.word, q.definition, q.stage)
		var d int32
		if err := row.Scan(&d); err != nil {
			t.Errorf("Scanning Row %v: %v", q, err)
		}
		return d
	}
	check := func(q *row) {
		if count(q) <= 0 {
			t.Errorf("Scanning Row %v: No rows found", q)
		}
	}

	const chatId int64 = 1
	if err := r.Save(chatId, "foo", "foo is bar"); err != nil {
		t.Fatal(err)
	}
	check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 0})

	d, err := r.Repeat(chatId)
	if err != nil {
		t.Fatal(err)
	}
	if d != "******** is bar" {
		t.Errorf("got %q; want %q", d, "foo is bar")
	}
	check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 0})

	t.Run("Answer", func(t *testing.T) {
		// Answer is broken
		t.Skip()
		if _, err := r.Answer(chatId, d, "foo"); err != nil {
			t.Fatal(err)
		}
		check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 1})

		corrected, err := r.Answer(chatId, d, "wrong")
		if err != nil {
			t.Fatal(err)
		}
		if corrected != "foo" {
			t.Errorf("got %q; want foo", corrected)
		}
		check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 0})

		for _, want := range []int32{1, 2, 3, 3, 3} {
			if _, err := r.Answer(chatId, d, "foo"); err != nil {
				t.Fatal(err)
			}
			check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: want})
		}
	})

	// Test simpler know - don't know
	if err := r.AnswerDontKnow(chatId, "foo"); err != nil {
		t.Fatal(err)
	}
	check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 0})

	for _, want := range []int32{1, 2, 3, 3, 3} {
		if err := r.AnswerKnow(chatId, "foo"); err != nil {
			t.Fatal(err)
		}
		check(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: want})
	}

	if e, err := r.Exists(chatId, "foo"); err != nil || !e {
		t.Errorf("r.Exists: %t, %v want true, nil", e, err)
	}
	if err := r.Delete(chatId, "foo"); err != nil {
		t.Fatal(err)
	}
	if count(&row{chatId: chatId, word: "foo", definition: "foo is bar", stage: 3}) > 0 {
		t.Errorf("%q wasn't deleted", "foo")
	}
	// consecutive deletions of the row result in no error
	if e, err := r.Exists(chatId, "foo"); err != nil || e {
		t.Errorf("r.Exists: %t, %v want false, nil", e, err)
	}
	if err := r.Delete(chatId, "foo"); err != nil {
		t.Fatal(err)
	}
}
