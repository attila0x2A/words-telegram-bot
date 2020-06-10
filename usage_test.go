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
	"strings"
	"testing"
)

func TestUsageFetcher(t *testing.T) {
	uf, err := NewUsageFetcher(UsageFetcherOptions{
		SentencesPath: "testdata/sentences.csv",
		LinksPath:     "testdata/links.csv",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, word := range []string{"fekete", "fehér", "közös"} {
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
			if len(ex) == 0 {
				t.Fatalf("len(usage examples): got %d; want > 0", len(ex))
			}
			for _, e := range ex {
				if !strings.Contains(e.Text, word) {
					t.Errorf("%q doesn't contain query word", e)
				}
			}
		})
	}
}
