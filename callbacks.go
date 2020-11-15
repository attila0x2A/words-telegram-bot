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

type KnowCallback struct {
	Word string
}

func (KnowCallback) Call(s *State, q *CallbackQuery) error {
	defer s.Telegram.AnswerCallbackLog(q.Id, "")
	chatID := q.Message.Chat.Id
	word := CallbackInfoFromString(q.Data).Word

	// TODO: Need to handle 2 rapid taps to avoid saving it as known 2 times in a row.
	if err := s.Repetitions.Answer(chatID, word, AnswerGood); err != nil {
		return err
	}

	if err := flipWordCard(s.Clients, word, q.Message, []*InlineKeyboard{DontKnowCallback{word, false}.AsInlineKeyboard()}); err != nil {
		return err
	}
	return practiceReply(s, chatID)
}

func (KnowCallback) Match(s *State, q *CallbackQuery) bool {
	info := CallbackInfoFromString(q.Data)
	return info.Action == PracticeKnowAction
}

func (k KnowCallback) AsInlineKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Know",
		CallbackData: CallbackInfo{
			Action: PracticeKnowAction,
			Word:   k.Word,
		}.String(),
	}
}

type DontKnowCallback struct {
	Word string
	// If true when clicking another practice card will be shown.
	Practice bool
}

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

func (DontKnowCallback) Match(_ *State, q *CallbackQuery) bool {
	info := CallbackInfoFromString(q.Data)
	return info.Action == PracticeDontKnowAction || info.Action == ResetProgressAction
}

func (c DontKnowCallback) AsInlineKeyboard() *InlineKeyboard {
	a := ResetProgressAction
	if c.Practice {
		a = PracticeDontKnowAction
	}
	return &InlineKeyboard{
		Text: "Don't know",
		CallbackData: CallbackInfo{
			Action: a,
			Word:   c.Word,
		}.String(),
	}
}

type ResetProgressCallback struct {
	Word string
}

// ResetProgress type is just a convenience placeholder to create inline keyboards.
// Logic will be handled by dont know callback.
func (ResetProgressCallback) Call(_ *State, _ *CallbackQuery) error {
	return nil
}

func (ResetProgressCallback) Match(_ *State, _ *CallbackQuery) bool {
	return false
}

func (c ResetProgressCallback) AsInlineKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Reset progress",
		CallbackData: CallbackInfo{
			Action: ResetProgressAction,
			Word:   c.Word,
		}.String(),
	}
}

type LearnCallback struct {
	Word string
}

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

func (LearnCallback) Match(_ *State, q *CallbackQuery) bool {
	info := CallbackInfoFromString(q.Data)
	return info.Action == SaveWordAction
}

func (c LearnCallback) AsInlineKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Learn",
		CallbackData: CallbackInfo{
			Action: SaveWordAction,
			Word:   c.Word,
		}.String(),
	}
}
