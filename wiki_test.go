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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseWiki(t *testing.T) {
	f, err := ioutil.ReadFile("testdata/test.html")
	if err != nil {
		t.Fatal(err)
	}
	parser := WikiParser{
		InputLanguage: "Hungarian",
	}
	got, err := parser.ParseWiki(string(f))
	if err != nil {
		t.Fatal(err)
	}

	want := []*WikiDefinition{
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black (absorbing all light and reflecting none)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black (pertaining to a dark-skinned ethnic group)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black (darker than other varieties, especially of fruits and drinks)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "(figuratively) tragic, mournful, black (causing great sadness or suffering)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "(figuratively) black (derived from evil forces, or performed with the intention of doing harm)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "(figuratively, in compounds) illegal (contrary to or forbidden by criminal law)",
			SpeechPart: "Adjective",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black (color perceived in the absence of light)",
			SpeechPart: "Noun",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black clothes (especially as mourning attire)",
			SpeechPart: "Noun",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "black person (member of a dark-skinned ethnic group)",
			SpeechPart: "Noun",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "dark-haired person (especially a woman with dark hair)",
			SpeechPart: "Noun",
		},
		&WikiDefinition{
			Word:       "fekete",
			Definition: "(colloquial) black coffee (coffee without cream or milk)",
			SpeechPart: "Noun",
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("ParseWiki: (-got +want):\n%s", diff)
	}
}
