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
// This is a script to load up usage examples into the database.
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/sync/errgroup"
)

type UsageFetcherOptions struct {
	// Path to the file in csv with links between files, <id><TAB><translation>
	LinksPath string
	// Path to the file in csv with all the sentences. <id><TAB><lang><TAB><text>
	// <lang> is an ISO 639-3 language code.
	SentencesPath string
}

type wordLang struct {
	word string
	lang string
}

type sentence struct {
	text string
	lang string
}

func (l *Loader) ReadAndLoad(opts UsageFetcherOptions) error {
	sf, err := os.Open(opts.SentencesPath)
	if err != nil {
		return err
	}
	defer sf.Close()

	// Use single proc so that tx is single. Count and flush (commit
	// transaction & create a new one) every 1M rows.
	// proc would have sentence, word, translation methods. queries will be
	// embedded.
	p, err := newProc(l)
	if err != nil {
		return err
	}
	defer p.cleanup()

	c := make(chan string)

	processSentence := func() error {
		for row := range c {
			s := strings.Split(row, "\t")
			if len(s) != 3 {
				return fmt.Errorf("reading %q: wrond format for row %s", opts.SentencesPath, s)
			}
			id, err := strconv.ParseInt(s[0], 10, 64)
			if err != nil {
				return fmt.Errorf("reading %q: parsing id %q: %v", opts.SentencesPath, s[0], err)
			}
			lang, text := s[1], s[2]
			if err := p.sentence(id, lang, text); err != nil {
				return err
			}
			r := strings.NewReplacer(
				",", "",
				".", "",
				"!", "",
				")", "",
				"(", "",
				"}", "",
				"{", "",
				"]", "",
				"[", "",
			)
			for _, w := range strings.Split(text, " ") {
				word := strings.ToLower(r.Replace(w))
				if err := p.word(word, lang, id); err != nil {
					return err
				}
			}
		}
		return nil
	}

	scanner := bufio.NewScanner(sf)
	go func() {
		for scanner.Scan() {
			c <- scanner.Text()
		}
		close(c)
	}()
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %q: %w", opts.SentencesPath, err)
	}
	eg := errgroup.Group{}
	for n := 0; n < 16; n++ {
		eg.Go(processSentence)
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	lf, err := os.Open(opts.LinksPath)
	if err != nil {
		return err
	}
	defer lf.Close()

	scanner = bufio.NewScanner(lf)
	for scanner.Scan() {
		var ids []int64
		for _, i := range strings.Split(scanner.Text(), "\t") {
			id, err := strconv.ParseInt(i, 10, 64)
			if err != nil {
				return fmt.Errorf("reading %q: parsing id %q: %v", opts.LinksPath, i, err)
			}
			ids = append(ids, id)
		}
		if len(ids) != 2 {
			return fmt.Errorf("reading %q: wrond format for row %s", opts.LinksPath, scanner.Text())
		}
		if err := p.translation(ids[0], ids[1]); err != nil {
			return err
		}
	}

	return nil
}

type Loader struct {
	db   *sql.DB
	opts UsageFetcherOptions
}

func NewLoader(dbPath, sPath, lPath string) (*Loader, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	return &Loader{
		db,
		UsageFetcherOptions{
			SentencesPath: sPath,
			LinksPath:     lPath,
		},
	}, nil
}

type proc struct {
	db   *sql.DB
	stmt map[TableType]*sql.Stmt

	mu        sync.Mutex
	cnt       map[TableType]int
	processed int64
	tx        *sql.Tx
}

type TableType int

const (
	SentencesTable TableType = iota
	WordsTable
	TranslationsTable
)

func newProc(l *Loader) (p *proc, err error) {
	p = new(proc)
	p.tx, err = l.db.Begin()
	if err != nil {
		return
	}
	p.db = l.db
	p.cnt = make(map[TableType]int)
	p.stmt = make(map[TableType]*sql.Stmt)
	for t, q := range map[TableType]string{
		SentencesTable: `INSERT OR REPLACE INTO Sentences(id, lang, text)
			VALUES(?, ?, ?)`,
		WordsTable: `INSERT OR REPLACE INTO Words(word, lang, sentence_id)
			VALUES(?, ?, ?)`,
		TranslationsTable: `INSERT OR REPLACE INTO Translations(id, translation_id)
			VALUES(?, ?)`,
	} {
		p.stmt[t], err = l.db.Prepare(q)
		if err != nil {
			return
		}
	}
	return
}

func (p *proc) sentence(id int64, lang, text string) error {
	err := p.row(SentencesTable, id, lang, text)
	if err != nil {
		err = fmt.Errorf("Row(%d, %s, %s): %v", id, lang, text, err)
	}
	return err
}
func (p *proc) word(word, lang string, sid int64) error {
	err := p.row(WordsTable, word, lang, sid)
	if err != nil {
		err = fmt.Errorf("Row(%s, %s, %d): %v", word, lang, sid, err)
	}
	return err
}
func (p *proc) translation(id, tid int64) error {
	err := p.row(TranslationsTable, id, tid)
	if err != nil {
		err = fmt.Errorf("Row(%d, %d): %v", id, tid, err)
	}
	return err
}

func (p *proc) row(table TableType, args ...interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cnt[table] += 1
	_, err := p.tx.Stmt(p.stmt[table]).Exec(args...)
	if err != nil {
		return err
	}
	p.processed += 1
	if p.processed%100_000 == 0 {
		return p.commit()
	}
	return nil
}

func (p *proc) commit() (err error) {
	log.Printf("Flushing %d rows", p.processed)
	if err := p.tx.Commit(); err != nil {
		return err
	}
	log.Printf("In total wrote %v", p.cnt)
	p.processed = 0
	p.tx, err = p.db.Begin()
	return err
}

func (p *proc) cleanup() {
	if err := p.commit(); err != nil {
		log.Printf("ERROR proc cleanup: %v", err)
	}
	for _, s := range p.stmt {
		s.Close()
	}
	p.tx.Rollback()
}

func (l *Loader) Load() error {
	// word -> list of sentences (ids). OR word -> lang -> list of sentences.
	// sentence id -> list of translation id.
	// translation id -> sentence.
	if _, err := l.db.Exec(`
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
	`); err != nil {
		return err
	}

	if err := l.ReadAndLoad(l.opts); err != nil {
		return err
	}

	//{
	//	p, err := newProc(l,
	//		`INSERT OR REPLACE INTO Sentences(id, lang, text)
	//		VALUES(?, ?, ?)`)
	//	if err != nil {
	//		return err
	//	}
	//	defer p.cleanup()
	return nil
}

func main() {
	db := flag.String("db_path", "../db.sql", "Path to the persistent sqlite3 database.")
	sentences := flag.String("sentences", "../data/sentences.csv", "Path to the folder with sentences usage examples in csv format.")
	links := flag.String("links", "../data/links.csv", "Path to the folder with links usage examples in csv format.")
	flag.Parse()

	l, err := NewLoader(*db, *sentences, *links)
	if err != nil {
		log.Fatal(err)
	}
	if err := l.Load(); err != nil {
		log.Fatal(err)
	}
}
