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
// This file contains logic for extracting word definitions
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type Definer struct {
	usage *UsageFetcher
	cache DefCacheInterface
	http  *http.Client
}

func (d *Definer) Define(word string, settings *Settings) (ds []string, err error) {
	// TODO: Not very elegant, is there a better way?
	const separator = "lsjelrzprhkvzvxzquhurhcakugvuhtqkrklggdpdseus----="
	_, def, err := d.cache.Lookup(word)
	if err == nil {
		return strings.Split(def, separator), nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		defer func() {
			// TODO: Make a use of corrected word once more structured
			// information is returned.
			if len(ds) == 0 || err != nil {
				return
			}
			if err := d.cache.Save(word, word, strings.Join(ds, separator)); err != nil {
				log.Printf("cache.Save(%q): %v", word, err)
			}
		}()
	} else {
		// At this point err != nil
		log.Printf("ERROR: cache.Lookup(%q): %v", word, err)
	}

	p := WikiParser{
		InputLanguage: settings.InputLanguage,
	}
	defs, err := FetchWikiDefinition(p, d.http, word)
	if err != nil {
		return nil, err
	}
	word = defs[0].Word

	ex, err := d.usage.FetchExamples(word, settings.InputLanguageISO639_3, settings.TranslationLanguages)
	if err != nil {
		ex = nil
		log.Printf("ERROR: FetchExamples(%s): %v", word, err)
		log.Printf("WARNING Did not find usage examples for %q", word)
	}
	msg := "*" + escapeMarkdown(word) + "*\n"
	for i, d := range defs {
		if i > 7 {
			msg += "\n"
			msg += fmt.Sprintf("_\\[truncated %d definitions\\]_", len(defs)-i)
			break
		}
		msg += "\n"
		msg += fmt.Sprintf(`%d\. \[*%s*\] %s`, i+1, strings.ToLower(d.SpeechPart), escapeMarkdown(d.Definition))
	}
	if len(ex) > 0 {
		msg += "\n\nUsage examples:"
		for i, e := range ex {
			msg += "\n\n"
			msg += fmt.Sprintf(`%d\. %s`, i+1, escapeMarkdown(e.Text))
			for _, t := range e.Translations {
				msg += "\n" + fmt.Sprintf(`  _%s_`, escapeMarkdown(t))
			}
		}
	} else {
		msg += escapeMarkdown("\n\nDidn't find usage examples.")
	}
	return []string{msg}, nil
}
