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
	"strings"
	"testing"
)

const usageSQL = `
	PRAGMA foreign_keys = OFF;

	CREATE TABLE IF NOT EXISTS Sentences (
		id INTEGER PRIMARY KEY,
		lang STRING,
		text STRING
	);

	CREATE TABLE IF NOT EXISTS Translations (
		id INTEGER,
		translation_id INTEGER,
		FOREIGN KEY(id) REFERENCES Sentences(id),
		FOREIGN KEY(translation_id) REFERENCES Sentences(id)
	);
	CREATE INDEX IF NOT EXISTS TranslationsIdIndex
	ON Translations (id);

	CREATE TABLE IF NOT EXISTS Words (
		word STRING,
		lang STRING,
		sentence_id INTEGER,
		FOREIGN KEY(sentence_id) REFERENCES Sentences(id)
	);
	CREATE INDEX IF NOT EXISTS WordLangIndex
	ON Words (word, lang);
	
	INSERT OR REPLACE INTO Sentences(id, lang, text) VALUES
		(1, "hun", "fekete kutya"),
		(2, "hun", "fekete disznó"),
		(3, "hun", "fekete macska fehér asztalon"),
		(4, "hun", "fehér disznó"),
		(5, "hun", "fehér fal"),
		(6, "hun", "fehér haj"),
		(7, "eng", "black dog"),
		(8, "eng", "white pig"),
		(9, "ukr", "чорний собака");
	INSERT OR REPLACE INTO Words(word, lang, sentence_id) VALUES
		("fekete", "hun", 1),
		("fekete", "hun", 2),
		("fekete", "hun", 3),
		("fehér", "hun", 3),
		("fehér", "hun", 4),
		("fehér", "hun", 5),
		("fehér", "hun", 6);
	INSERT OR REPLACE INTO Translations(id, translation_id) VALUES
		(1, 7),
		(1, 9),
		(7, 1),
		(9, 1),
		(4, 8),
		(8, 4);
	`

func TestUsageFetcher(t *testing.T) {
	dir, err := ioutil.TempDir("", "repetition")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Temp dir: %q", dir)
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "tmpdb")
	uf, err := NewUsageFetcher(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uf.db.Exec(usageSQL); err != nil {
		t.Fatal(err)
	}

	for _, word := range []string{"fekete", "fehér"} {
		t.Run(word, func(t *testing.T) {
			ex, err := uf.FetchExamples(word, "hun", map[string]bool{
				"eng": true,
				"rus": true,
				"ukr": true,
			})
			if err != nil {
				t.Fatal(err)
			}
			// check that each returned example contains asked query.
			if len(ex) != 3 {
				t.Fatalf("len(usage examples): got %d; want 3", len(ex))
			}
			for _, e := range ex {
				if !strings.Contains(e.Text, word) {
					t.Errorf("%q doesn't contain query word", e)
				}
			}
		})
	}
}
