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

import "fmt"

// TODO: For anki structure:
// Add button - show - show back of the card.
//	* after show 4 buttons will appear (again, hard, good, easy)
//	* all 4 buttons can be similar. Code should be very similar.
//	* new card appears after again, hard, good, easy is chosen.
// reset progress should probably be renamed to practise sooner.
//	* equivalent to again.

type KnowCallback struct{}

func (KnowCallback) Call(s *State, q *CallbackQuery) error {
	defer s.Telegram.AnswerCallbackLog(q.Id, "")
	chatID := q.Message.Chat.Id
	word := CallbackInfoFromString(q.Data).Word

	// TODO: Need to handle 2 rapid taps to avoid saving it as known 2 times in a row.
	if err := s.Repetitions.Answer(chatID, word, AnswerGood); err != nil {
		return err
	}

	if err := flipWordCard(s.Clients, word, q.Message, []*InlineKeyboard{resetProgressIK(word)}); err != nil {
		return err
	}
	return practiceReply(s, chatID)
}

func knowIK(word string) *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Know",
		CallbackData: CallbackInfo{
			Action: PracticeKnowAction,
			Word:   word,
		}.String(),
	}
}

type DontKnowCallback struct{}

func (DontKnowCallback) Call(s *State, q *CallbackQuery) error {
	defer s.Telegram.AnswerCallbackLog(q.Id, "Reset progress")
	info := CallbackInfoFromString(q.Data)
	chatID := q.Message.Chat.Id
	word := info.Word

	if err := s.Repetitions.Answer(chatID, word, AnswerAgain); err != nil {
		return err
	}

	if err := flipWordCard(s.Clients, word, q.Message, nil); err != nil {
		return err
	}

	if info.Action == ResetProgressAction {
		return nil
	}
	return practiceReply(s, chatID)
}

func dontKnowIK(word string) *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Don't know",
		CallbackData: CallbackInfo{
			Action: PracticeDontKnowAction,
			Word:   word,
		}.String(),
	}
}

func resetProgressIK(word string) *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Reset progress",
		CallbackData: CallbackInfo{
			Action: ResetProgressAction,
			Word:   word,
		}.String(),
	}
}

type LearnCallback struct{}

func (LearnCallback) Call(s *State, q *CallbackQuery) error {
	// FIXME: Next 3 lines are very common.
	chatID := q.Message.Chat.Id
	word := CallbackInfoFromString(q.Data).Word
	if err := s.Repetitions.Save(chatID, word, q.Message.Text); err != nil {
		return err
	}
	m := q.Message
	r := &EditMessageText{
		ChatId:    m.Chat.Id,
		MessageId: m.Id,
		ReplyMarkup: ReplyMarkup{
			InlineKeyboard: [][]*InlineKeyboard{
				[]*InlineKeyboard{},
			},
		},
	}
	var rm Message
	if err := s.Telegram.Call("editMessageReplyMarkup", r, &rm); err != nil {
		return fmt.Errorf("editing message reply markup: %w", err)
	}
	msg := fmt.Sprintf("Saved %q for learning", word)
	s.Telegram.AnswerCallbackLog(q.Id, msg)
	return nil
}

func learnIK(word string) *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Learn",
		CallbackData: CallbackInfo{
			Action: SaveWordAction,
			Word:   word,
		}.String(),
	}
}
