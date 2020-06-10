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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSettings(t *testing.T) {
	dir, err := ioutil.TempDir("", "repetition")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Temp dir: %q", dir)
	defer os.RemoveAll(dir)

	db := filepath.Join(dir, "tmpdb")
	settings, err := NewSettingsConfig(db)
	if err != nil {
		t.Fatal(err)
	}

	var chatID int64 = 0
	s, err := settings.Get(chatID)
	if err != nil {
		t.Error(err)
	}
	if err := settings.Set(chatID, s); err != nil {
		t.Error(err)
	}

	s.InputLanguage = "foo_bar"
	if err := settings.Set(chatID, s); err != nil {
		t.Error(err)
	}
	ns, err := settings.Get(chatID)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(s, ns) {
		t.Errorf("new settings were not set! old: %v\n new: %v", s, ns)
	}
}
