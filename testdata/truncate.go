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
// Truncate usage dataset for tests.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	// comma separated whitelisted words. Entries that contain these works will
	// be kept.
	// flags.String
	// input sentences
	// flags.String
	// input links
	// flags.String
	// output directory

	const (
		sentences = "../data/sentences.csv"
		links     = "../data/links.csv"
		out       = "./"
		keepList  = "fekete,black,falu,village,fehér,white,közös,common"
	)
	keep := make(map[string]bool)
	for _, k := range strings.Split(keepList, ",") {
		keep[k] = true
	}

	sf, err := os.Open(sentences)
	if err != nil {
		panic(err)
	}
	defer sf.Close()
	outS, err := os.Create(path.Join(out, "sentences.csv"))
	if err != nil {
		panic(err)
	}
	defer outS.Close()

	scanner := bufio.NewScanner(sf)
	wexp := regexp.MustCompile(`(,.!?;:")+`)
	ids := make(map[int64]bool)
	for scanner.Scan() {
		row := scanner.Text()
		k := false
		parts := strings.Split(row, "\t")
	SEARCH:
		for _, p := range parts {
			for _, w := range strings.Split(p, " ") {
				if keep[wexp.ReplaceAllString(w, "")] {
					k = true
					break SEARCH
				}
			}
		}
		if k {
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				panic(err)
			}
			ids[id] = true
			fmt.Fprintln(outS, row)
		}
	}

	fmt.Println("Starting processing links")

	lf, err := os.Open(links)
	if err != nil {
		panic(err)
	}
	defer lf.Close()
	outL, err := os.Create(path.Join(out, "links.csv"))
	if err != nil {
		panic(err)
	}
	defer outL.Close()

	scanner = bufio.NewScanner(lf)
	for scanner.Scan() {
		row := scanner.Text()
		id, err := strconv.ParseInt(strings.Split(row, "\t")[0], 10, 64)
		if err != nil {
			panic(err)
		}
		if ids[id] {
			fmt.Fprintln(outL, row)
		}
	}
}
