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
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type UsageFetcherOptions struct {
	// Path to the file in csv with links between files, <id><TAB><translation>
	LinksPath string
	// Path to the file in csv with all the sentences. <id><TAB><lang><TAB><text>
	// <lang> is an ISO 639-3 language code.
	SentencesPath string
}

// Usage is struct that is able to extract usage examples from the tatoeba
// datasets.
type UsageFetcher struct {
	// word -> sentence IDs
	ws map[string][]int64
	// id -> sentence
	ss map[int64]*sentence
	// id -> translation ids
	ts map[int64][]int64
}

type sentence struct {
	text           string
	lang           string
	translationIDs []int64
}

func NewUsageFetcher(opts UsageFetcherOptions) (*UsageFetcher, error) {
	// FIXME: Check that Language is not in TranslationLanguages
	sf, err := os.Open(opts.SentencesPath)
	if err != nil {
		return nil, err
	}
	defer sf.Close()

	whitelist := make(map[string]bool)
	for _, v := range SupportedInputLanguages {
		whitelist[v.InputLanguageISO639_3] = true
		for k, v := range v.TranslationLanguages {
			if v {
				whitelist[k] = true
			}
		}
	}

	ss := make(map[int64]*sentence)
	ws := make(map[string][]int64)
	scanner := bufio.NewScanner(sf)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), "\t")
		if len(s) != 3 {
			return nil, fmt.Errorf("reading %q: wrond format for row %s", opts.SentencesPath, s)
		}
		id, err := strconv.ParseInt(s[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("reading %q: parsing id %q: %v", opts.SentencesPath, s[0], err)
		}
		// Skip unsupported languages and translations for optimization. Might
		// need to be removed if this thing is made more general.
		if !whitelist[s[1]] {
			continue
		}
		ss[id] = &sentence{
			lang: s[1],
			text: s[2],
		}
		// FIXME: Words will not be correctly extracted if there are
		// punctuations.
		for _, w := range strings.Split(s[2], " ") {
			ws[w] = append(ws[w], id)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %q: %w", opts.SentencesPath, err)
	}

	lf, err := os.Open(opts.LinksPath)
	if err != nil {
		return nil, err
	}
	defer lf.Close()

	ts := make(map[int64][]int64)
	scanner = bufio.NewScanner(lf)
	for scanner.Scan() {
		var ids []int64
		for _, i := range strings.Split(scanner.Text(), "\t") {
			id, err := strconv.ParseInt(i, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("reading %q: parsing id %q: %v", opts.LinksPath, i, err)
			}
			ids = append(ids, id)
		}
		if len(ids) != 2 {
			return nil, fmt.Errorf("reading %q: wrond format for row %s", opts.LinksPath, scanner.Text())
		}
		ts[ids[0]] = append(ts[ids[0]], ids[1])
	}
	log.Printf("Number of translations: %d", len(ts))

	return &UsageFetcher{
		ws: ws,
		ss: ss,
		ts: ts,
	}, nil
}

type UsageExample struct {
	Text         string
	Translations []string
}

// FIXME: Too many parameters
// language is a langugage of the word in ISO 639-3 format.
func (u *UsageFetcher) FetchExamples(word, language string, translationLanguages map[string]bool) ([]*UsageExample, error) {
	// TODO: rank examples by complexity and extract the simplest ones:
	// 1) for each word calculate it's complexity by the number of sentences it's
	// used in (more sentences -> simpler words)
	// 2) the sentence is simpler if it contains simpler words. Maybe average
	// word simplicity to not disqualify long sentences.
	//
	// TODO: Prioritize using sentences with the most translations.
	var ex []*UsageExample
	for _, i := range u.ws[word] {
		s := u.ss[i]
		if s.lang != language {
			continue
		}
		var tr []string
		for _, i := range u.ts[i] {
			s, ok := u.ss[i]
			if !ok {
				log.Printf("ERROR: inconsistent sentences, no id %d found in sentences", i)
				continue
			}
			if _, ok := translationLanguages[s.lang]; !ok {
				continue
			}
			tr = append(tr, s.text)
		}
		if len(tr) > 4 {
			tr = tr[:4]
		}
		ex = append(ex, &UsageExample{
			Text:         s.text,
			Translations: tr,
		})
	}
	if len(ex) == 0 {
		return nil, fmt.Errorf("no sentences match %s", word)
	}
	sort.Slice(ex, func(i, j int) bool { return len(ex[i].Translations) > len(ex[j].Translations) })
	if len(ex) > 3 {
		ex = ex[:3]
	}
	return ex, nil
}
