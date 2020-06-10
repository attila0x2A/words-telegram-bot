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
	"log"
	"strings"
	"time"
)

type Repetition struct {
	db *sql.DB
	// FIXME: Probably not needed here. Maybe only the number of stages.
	stages []time.Duration
}

func NewRepetition(dbPath string, stages []time.Duration) (*Repetition, error) {
	// this is arbitrary big number
	const maxStages = 1_000_000
	if len(stages) == 0 {
		panic("stages == 0")
	}
	if len(stages) >= maxStages {
		panic(fmt.Sprintf("too many stages; should be less than %d", maxStages))
	}
	var sv []string
	for k, s := range stages {
		sv = append(sv, fmt.Sprintf("(%d, %d)", k, int64(s.Seconds())))
	}
	// insert large last id so that words with stages > len(stages) can still
	// be queried (This can happen if number of stages shrinks)
	sv = append(sv, fmt.Sprintf("(%d, %d)", maxStages, int64(stages[len(stages)-1].Seconds())))
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS Repetition (
			chat_id INTEGER,
			word STRING,
			definition STRING,
			stage INTEGER,
			last_updated_seconds INTEGER -- seconds since UNIX epoch
		);
		CREATE TEMP TABLE IF NOT EXISTS Stages (
			id INTEGER,
			duration INTEGER
		);
		INSERT INTO Stages(id, duration)
			VALUES ` +
		// Usually not escaping sql parts can lead to sql injection. In
		// this case it's more convenient, and only numbers are put inside.
		strings.Join(sv, ","),
	); err != nil {
		return nil, err
	}
	row := db.QueryRow(`
		SELECT COUNT(*)
		FROM Repetition;`)
	var d int
	if err := row.Scan(&d); err != nil {
		return nil, err
	}
	log.Printf("DEBUG: Repetition database initially contains %d rows!", d)
	return &Repetition{db, stages}, nil
}

func (r *Repetition) Save(chatID int64, word, definition string) error {
	// FIXME: Don't insert duplicates!!!
	_, err := r.db.Exec(`
		INSERT INTO Repetition(chat_id, word, definition, stage, last_updated_seconds)
		VALUES($0, $1, $2, $3, $4)`,
		chatID, word, definition, 0, time.Now().Unix())
	return err
}

// Repeat retrieves a definitions of the word ready for repetition.
func (r *Repetition) Repeat(chatID int64) (string, error) {
	// TODO: Can consider ordering by oldest
	// TODO: Add a test for this somehow to make sure that correct amount of
	// time is waited. (can modify last_updated_seconds inside the test to
	// simulate time)
	row := r.db.QueryRow(`
		SELECT word, definition
		FROM Repetition
		INNER JOIN Stages ON Repetition.stage <= Stages.id
		WHERE Repetition.last_updated_seconds + Stages.duration <= $0
		  AND Repetition.chat_id = $1;`,
		time.Now().Unix(), chatID)
	var w, d string
	err := row.Scan(&w, &d)
	if err != nil {
		return "", err
	}
	// strip first paragraph which corresponds to the word in question.
	if s := strings.Split(d, "\n\n"); len(s) > 1 {
		d = strings.Join(s[1:], "\n\n")
	}
	// Make sure that the word is not in the question.
	return strings.ReplaceAll(d, w, "********"), nil
}

// Repeat retrieves a word ready for repetition.
// TODO: Deduplicate with Repeat?
func (r *Repetition) RepeatWord(chatID int64) (string, error) {
	row := r.db.QueryRow(`
		SELECT word
		FROM Repetition
		INNER JOIN Stages ON Repetition.stage <= Stages.id
		WHERE Repetition.last_updated_seconds + Stages.duration <= $0
		  AND Repetition.chat_id = $1;`,
		time.Now().Unix(), chatID)
	var w string
	err := row.Scan(&w)
	return w, err
}

// looks up definition and compares it to the word
// FIXME: FIXME: FIXME: FIXME: This doesn't work!!!!!!!!
//  cannot save obfuscated - cannot check.
//  cannot save clear - cannot extract raw from obfuscated.
//  this need fixing - make sure repetition_test passes.
//  a way to fix is to move obfuscation into commander, save into Asking
//    not-obfuscated message, but send to user obfuscated one.
// Maybe this is already fixed, just not tested?
func (r *Repetition) Answer(chatID int64, definition, word string) (string, error) {
	panic("This logic is broken, fix it!")
	row := r.db.QueryRow(`
		SELECT word, stage
		FROM Repetition
		WHERE definition = $0
		  AND chat_id = $1`,
		definition, chatID)
	var correct string
	var stage int
	if err := row.Scan(&correct, &stage); err != nil {
		return "", fmt.Errorf("INTERNAL: Did not find definition %q: %w", definition, err)
	}
	if correct != word {
		stage = 0
	} else {
		stage += 1
		if stage >= len(r.stages) {
			stage = len(r.stages) - 1
		}
	}
	_, err := r.db.Exec(`
		UPDATE Repetition
		SET stage = $0, last_updated_seconds = $1
		WHERE definition = $2
		  AND chat_id = $3;`,
		stage, time.Now().Unix(), definition, chatID)
	if err != nil {
		return "", fmt.Errorf("INTERNAL: Failed updating stage: %w", err)
	}
	return correct, nil
}

func (r *Repetition) AnswerKnow(chatID int64, word string) error {
	_, err := r.db.Exec(`
		UPDATE Repetition
		SET stage = MIN(stage + 1, $0), last_updated_seconds = $1
		WHERE word = $2
		  AND chat_id = $3;`,
		len(r.stages)-1, time.Now().Unix(), word, chatID)
	if err != nil {
		return fmt.Errorf("INTERNAL: Failed updating stage: %w", err)
	}
	return nil
}

func (r *Repetition) AnswerDontKnow(chatID int64, word string) error {
	_, err := r.db.Exec(`
		UPDATE Repetition
		SET stage = 0, last_updated_seconds = $0
		WHERE word = $1
		  AND chat_id = $2;`,
		time.Now().Unix(), word, chatID)
	if err != nil {
		return fmt.Errorf("INTERNAL: Failed updating stage: %w", err)
	}
	return nil
}

func (r *Repetition) GetDefinition(chatID int64, word string) (string, error) {
	row := r.db.QueryRow(`
		SELECT definition
		FROM Repetition
		WHERE word = $0
		  AND chat_id = $1`,
		word, chatID)
	var d string
	if err := row.Scan(&d); err != nil {
		return "", fmt.Errorf("INTERNAL: Did not find definition: %w", err)
	}
	return d, nil
}

func (r *Repetition) Exists(chatID int64, word string) (bool, error) {
	row := r.db.QueryRow(`
			SELECT COUNT(*) FROM Repetition
			WHERE chat_id = $1
				AND word = $2`,
		chatID, word)
	var d int32
	if err := row.Scan(&d); err != nil {
		return false, fmt.Errorf("INTERNAL: Counting %q for chat %d: %w", word, chatID, err)
	}
	return d > 0, nil
}

func (r *Repetition) Delete(chatID int64, word string) error {
	_, err := r.db.Exec(`
		DELETE
		FROM Repetition
		WHERE word = $0
		  AND chat_id = $1`,
		word, chatID)
	if err != nil {
		return fmt.Errorf("Failed deleting %q: %w", word, err)
	}
	return nil
}

// TODO later editing should be helpful.
// func (r *Repetition) Edit(chatID int64, word, newDefinition string) {
// }
