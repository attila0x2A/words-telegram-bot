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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var wikiUrlPrefix = "https://en.wiktionary.org/w/api.php"

type WikiDefinition struct {
	Word       string
	Definition string
	SpeechPart string // FIXME: Can be an enum
	// ?? Synonyms   []string
	// ?? Antonyms   []string
	// ?? Etymology
	// ?? Derivied terms
	// ?? Expressions
	// ?? Declension & Conjugations
	// ?? Source URL? probably populated not here.
}

type WikiParser struct {
	InputLanguage string
}

// FIXME: Should accept json instead and extract html here?
func (w WikiParser) ParseWiki(text string) ([]*WikiDefinition, error) {
	m, s, err := w.parseWikiHTML(text)
	if err != nil {
		return nil, err
	}
	log.Printf("subsections: %v", s)

	whitelisted := func(s string) bool {
		whitelist := []string{"Noun", "Verb", "Adjective", "Adverb", "Pronoun", "Preposition", "Conjunction"}
		for _, w := range whitelist {
			if strings.HasPrefix(s, w) {
				return true
			}
		}
		return false
	}

	var defs []*WikiDefinition
	for _, n := range s[w.InputLanguage] {
		if !whitelisted(n) {
			log.Printf("Ignoring %q, not whitelisted", n)
			continue
		}
		r := m[n]
		if r == "" {
			r = n + ": no definitions found"
		}
		defs = append(defs, w.extractDefs(r)...)
	}
	return defs, nil
}

// extractDefs extracts what it can from one chunk of text corresponding to
// definition.
// It assume following structure:
// <Part of speach>
// <word> (<addition information>)
//
// <def1>
//
// <def2>
func (WikiParser) extractDefs(text string) []*WikiDefinition {
	lines := strings.Split(text, "\n\n")
	if len(lines) < 2 {
		log.Printf("ERROR parsing %s: too few lines", text)
		return nil
	}
	pl := strings.Split(lines[0], "\n")
	if len(pl) < 2 {
		log.Printf("ERROR parsing word and part of speech %s: too few lines", lines[0])
	}
	p := pl[0]
	var w string
	if ws := strings.Split(pl[1], " "); len(ws) > 0 {
		w = ws[0]
	}

	var d []*WikiDefinition
	for _, ll := range lines[1:] {
		if s := strings.TrimSpace(ll); len(s) > 0 {
			d = append(d, &WikiDefinition{
				Word:       w,
				SpeechPart: p,
				Definition: s,
			})
		}
	}
	return d
}

// FIXME: Remove this nonsence probably?
const DebugWikiParser = false

// parseWikiHTML returns map section -> content and section -> []subsections; section key is id.
func (w WikiParser) parseWikiHTML(h string) (ms map[string]string, subs map[string][]string, err error) {
	if DebugWikiParser {
		// save in tmp location latest parsed file
		const file = "/tmp/html"
		if err = ioutil.WriteFile(file, []byte(h), 0644); err != nil {
			return
		}
		log.Printf("Written debug html to %s", file)
	}

	doc, err := html.Parse(strings.NewReader(h))
	if err != nil {
		return nil, nil, err
	}

	subs = make(map[string][]string)
	ms = make(map[string]string)

	parseTOC := func(n *html.Node) {
		// if this is a extract it's href, stripping leadind '#'
		href := func(n *html.Node) string {
			if n.Type != html.ElementNode || n.Data != "a" {
				return ""
			}
			for _, a := range n.Attr {
				if a.Key == "href" {
					return strings.TrimPrefix(a.Val, "#")
				}
			}
			return ""
		}
		// with visited, only immediate children are returned. It's more convenient to have all descendants included, even though it's more redundant info.
		//visited := make(map[*html.Node]bool)
		var f func(*html.Node, string)
		f = func(n *html.Node, p string) {
			//if visited[n] {
			//	return
			//}
			//visited[n] = true
			if l := href(n); l != "" {
				if p != "" {
					subs[p] = append(subs[p], l)
				}
				// leafs are included in the subs, as it's used to filter key ids
				subs[l] = nil
				for s := n.NextSibling; s != nil; s = s.NextSibling {
					f(s, l)
				}
			} else {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					f(c, p)
				}
			}
		}
		// parent and list with children are siblings
		f(n, "")
	}
	var f func(*html.Node)
	var contents string
	var lastId string
	f = func(n *html.Node) {
		// If id = toc - parse table of content to form subsection structure.
		if n.Type == html.ElementNode {
			if n.Data == "li" {
				// mark new definition with additional new line
				contents += "\n"
			}
			for _, a := range n.Attr {
				// ignore citation nodes
				if a.Key == "class" && a.Val == "citation-whole" {
					return
				}
				if a.Key != "id" {
					continue
				}
				if a.Val == "toc" {
					parseTOC(n)
					return
				}
				if _, ok := subs[a.Val]; ok {
					ms[lastId] = contents
					lastId = a.Val
					contents = ""
				}
			}
		} else if n.Type == html.TextNode {
			// Should keep the content only from ol? lists?
			contents += n.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	if lastId != "" {
		ms[lastId] = contents
	}
	return ms, subs, nil
}

// extract extracts parts of the json parsed v. If there are arrays on the left array is built and returned.
// NOTE: Reflection is slow.
func extract(f string, i interface{}) ([]interface{}, error) {
	flatten := func(vs []interface{}) []interface{} {
	FLATTEN:
		for {
			var newVs []interface{}
			for _, v := range vs {
				nvs, ok := v.([]interface{})
				// assume if a single one is not array none is array.
				if !ok || len(nvs) == 0 {
					break FLATTEN
				}
				newVs = append(newVs, nvs...)
			}
			vs = newVs
		}
		return vs
	}

	var vs []interface{}
	switch t := i.(type) {
	case []interface{}:
		vs = t
	default:
		vs = []interface{}{i}
	}

	for _, name := range strings.Split(f, ".") {
		vs = flatten(vs)
		var newVs []interface{}
		for _, v := range vs {
			t, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected type %v", v)
			}
			i, ok := t[name]
			if !ok {
				return nil, fmt.Errorf("extract(%q) %q not found", f, name)
			}
			newVs = append(newVs, i)
		}
		vs = newVs
	}
	return flatten(vs), nil
}

type Extractor struct {
	err error
}

func (e *Extractor) Extract(f string, i interface{}) (r []interface{}) {
	if e.err != nil || i == nil {
		return nil
	}
	r, e.err = extract(f, i)
	if e.err == nil && r == nil {
		e.err = fmt.Errorf("format %q became nil", f)
	}
	return r
}

func (e *Extractor) Extract1(f string, i interface{}) interface{} {
	r := e.Extract(f, i)
	if len(r) != 1 && e.err == nil {
		e.err = fmt.Errorf("format %q unexpected length %d", f, len(r))
		return nil
	}
	return r[0]
}

// FIXME: Might make sense to have additional information from which language
// wikipedia to extract data.
// Queries, parses one by one result until some definitions are found.
func FetchWikiDefinition(parser WikiParser, c *http.Client, w string) ([]*WikiDefinition, error) {
	get := func(p map[string]string) (_ string, err error) {
		q, err := http.NewRequest("GET", wikiUrlPrefix, nil)
		if err != nil {
			return
		}
		v := url.Values{}
		for k, pp := range p {
			v.Add(k, pp)
		}
		q.URL.RawQuery = v.Encode()
		log.Printf("DEBUG RawQuery: %q", q.URL.RawQuery)

		resp, err := c.Do(q)
		if err != nil {
			return "", fmt.Errorf("%s: %w", q.URL.RawQuery, err)
		}
		// TODO: This is not the most optimal way to decode it. Extra pass over bytes for json parsing.
		b := new(bytes.Buffer)
		if _, err = b.ReadFrom(resp.Body); err != nil {
			return
		}
		return b.String(), nil
	}

	resp, err := get(map[string]string{
		"action":   "query",
		"format":   "json",
		"list":     "search",
		"srsearch": w,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("DEBUG: Search results: %s", resp)
	var i interface{}
	if err := json.Unmarshal([]byte(resp), &i); err != nil {
		return nil, err
	}
	e := new(Extractor)
	ti := e.Extract("query.search.title", i)
	if len(ti) == 0 || e.err != nil {
		log.Printf("DEBUG: query.search.title : %v", err)
		return nil, errors.New("No search results")
	}

	var defs []*WikiDefinition
	for _, tti := range ti {
		title := tti.(string)
		log.Printf("DEBUG: Considering search result: %s", title)

		// Extract all the section.
		resp, err = get(map[string]string{
			"action":             "parse",
			"format":             "json",
			"prop":               "text",
			"disableeditsection": "true",
			"sectionpreview":     "true",
			"page":               title,
		})
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(resp), &i); err != nil {
			return nil, err
		}

		// TODO: Improve error handling. Bad requests happen, panic is bad.
		// FIXME: Should these be an explicit maybe with error checks on access?
		text := e.Extract1("parse.text.*", i).(string)
		if e.err != nil {
			return nil, e.err
		}
		wd, err := parser.ParseWiki(text)
		if err != nil {
			return nil, err
		}
		defs = append(defs, wd...)
		if len(defs) > 0 {
			break
		}
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("No definitions found for %q", w)
	}
	return defs, nil
}
