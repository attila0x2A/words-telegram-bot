This is not an officially supported Google product!

This is a telegram bot to help learn words through spaced repetitions.

It dynamically fetches definitions from wiktionary, and uses tatoeba database
for usage examples.

Sentence data can be downloaded from tatoeba and is not included in the git
repository. It should live in data/links.csv and data/sentences.csv

It can be downloaded from here:
https://tatoeba.org/eng/downloads

## QuickStart:
1. Create a telegram bot using @BotFather if you don't have one yet
2. Create secret.go in the root folder with the following content
```go

package main

const BotToken = "TOKEN PROVIDED BY BOT FATHER"
```
3. Download links and senteces from https://tatoeba.org/eng/downloads
4. Put fetched csv data under `./data`
5. Run using one of the following:
- using Go: `go build && ./words`
- using Docker:
```bash
sudo docker volume create words-vol
sudo docker build -t words .
sudo docker run --rm --name words-app --mount source=words-vol,target=/words-vol/db/ words
```
