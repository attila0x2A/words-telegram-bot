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
	"strings"
)

// Usage is struct that is able to extract usage examples from the tatoeba
// datasets.
type UsageFetcher struct {
	db *sql.DB
}

type sentence struct {
	text string
	lang string
}

// NewUsageFetcher creates a new usage fetcher.
func NewUsageFetcher(dbPath string) (*UsageFetcher, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	// Schema for the db can be found in migrate/load.go
	return &UsageFetcher{
		db: db,
	}, nil
}

type UsageExample struct {
	Text         string
	Translations []string
}

// FIXME: Too many parameters
// language is a langugage of the word in ISO 639-3 format.
func (u *UsageFetcher) FetchExamples(word, language string, translationLanguages map[string]bool) ([]*UsageExample, error) {
	var tls []interface{}
	for k, v := range translationLanguages {
		if v {
			tls = append(tls, k)
		}
	}
	// We use Sprintf only to insert variable number of ?, so it cannot cause
	// SQL injection.
	q := fmt.Sprintf(`
			SELECT DISTINCT s.text, ts.text
			FROM
				Words
			INNER JOIN
				Sentences s ON Words.sentence_id = s.id
			LEFT JOIN
				Translations ON Words.sentence_id = Translations.id
			LEFT JOIN
				Sentences ts ON Translations.translation_id = ts.id
			WHERE
			Words.word = ?
			AND s.lang = ?
			AND (ts.lang IS NULL OR ts.lang IN (?%s))
		-- If possible get definitions with translations first.
		ORDER BY CASE WHEN ts.text IS NULL THEN 1 ELSE 0 END
		LIMIT 3;`, strings.Repeat(", ?", len(tls)-1))
	args := append([]interface{}{
		word, language,
	}, tls...)
	rows, err := u.db.Query(q, args...)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no sentences match %s", word)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ex []*UsageExample
	for rows.Next() {
		var (
			e string
			t sql.NullString
		)
		if err := rows.Scan(&e, &t); err != nil {
			return nil, err
		}
		var tr []string
		if t.Valid {
			tr = append(tr, t.String)
		}
		ex = append(ex, &UsageExample{
			Text:         e,
			Translations: tr,
		})
	}

	// TODO: rank examples by complexity and extract the simplest ones:
	// 1) for each word calculate it's complexity by the number of sentences it's
	// used in (more sentences -> simpler words)
	// 2) the sentence is simpler if it contains simpler words. Maybe average
	// word simplicity to not disqualify long sentences.
	//
	// TODO: Prioritize using sentences with the most translations.
	return ex, nil
}
