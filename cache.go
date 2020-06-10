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
)

type DefCacheInterface interface {
	Lookup(q string) (word string, def string, err error)
	Save(q, w, d string) error
}

type NoCache struct {
}

func (*NoCache) Lookup(string) (string, string, error) {
	return "", "", sql.ErrNoRows
}

func (*NoCache) Save(_, _, _ string) error {
	return nil
}

type DefCache struct {
	db *sql.DB
}

// NewDefCache create DefCache using path to the database, creates a database
// if it doesn't exist already.
// FIXME: db should created in main and passed over here. (because it should be easy to replace it)
func NewDefCache(path string) (*DefCache, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// TODO: How schema changes would work?
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS Definitions (
			query string UNIQUE NOT NULL, -- user's query
			word string, -- the corresponding word (can be different from query in case of typos)
			definition string);
	`); err != nil {
		return nil, err
	}
	return &DefCache{db}, nil
}

// Lookup returns possible corrected word with it's definition
func (c *DefCache) Lookup(q string) (string, string, error) {
	row := c.db.QueryRow("SELECT word, definition FROM Definitions WHERE query = $1", q)
	var w, d string
	err := row.Scan(&w, &d)
	return w, d, err
}

func (c *DefCache) Save(q, w, d string) error {
	_, err := c.db.Exec(`INSERT INTO Definitions(query, word, definition)
		VALUES($0, $1, $2)`, q, w, d)
	return err
}
