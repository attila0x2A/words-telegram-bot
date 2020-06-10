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
//
//
// This is a adhoc one-time run script to perform data migration on the
// database.
package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func Migrate(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open: %v", err)
	}

	rows, err := db.Query(`
		SELECT word, definition
		FROM Repetition;`)
	defer rows.Close()

	nd := make(map[string]string)
	for rows.Next() {
		var w, d string
		if err := rows.Scan(&w, &d); err != nil {
			return fmt.Errorf("scan: %v", err)
		}
		nd[w] = strings.ReplaceAll(d, "********", w)
		if !strings.HasPrefix(d, w) && !strings.HasPrefix(d, "*"+w) {
			nd[w] = "*" + w + "*\n\n" + nd[w]
		}
	}
	rows.Close()

	for w, d := range nd {
		_, err := db.Exec(`
			UPDATE Repetition
			SET definition = $0
			WHERE word = $1;`,
			d, w)
		if err != nil {
			return fmt.Errorf("update: %v", err)
		}
	}
	return nil
}

func main() {
	fmt.Printf("Result of migration: %v\n", Migrate("../db.sql"))
}
