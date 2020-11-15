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
	"fmt"
)

type ShowAnswerCallback struct{}

func (ShowAnswerCallback) Call(s *State, q *CallbackQuery) error {
	defer s.Telegram.AnswerCallbackLog(q.Id, "")
	chatID := q.Message.Chat.Id
	ci := CallbackInfoFromString(q.Data)
	word := ci.Word

	var ik []*InlineKeyboard
	for _, ease := range []AnswerEase{AnswerAgain, AnswerHard, AnswerGood, AnswerEasy} {
		sc, err := s.Repetitions.CalcSchedule(chatID, word, ease)
		if err != nil {
			return err
		}
		ik = append(ik, answerIK(word, ease, sc.ivl))
	}
	return flipWordCard(s.Clients, word, q.Message, ik)
}

func showAnswerIK(word string) *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Show Answer",
		CallbackData: CallbackInfo{
			Action: ShowAnswerAction,
			Word:   word,
		}.String(),
	}
}

type AnswerCallback struct{}

func (AnswerCallback) Call(s *State, q *CallbackQuery) error {
	defer s.Telegram.AnswerCallbackLog(q.Id, "")
	chatID := q.Message.Chat.Id
	ci := CallbackInfoFromString(q.Data)
	word := ci.Word
	ease := ci.Ease

	// FIXME: Need to handle 2 rapid taps to avoid answering it 2 times in a row.
	if err := s.Repetitions.Answer(chatID, word, ease); err != nil {
		return err
	}

	// FIXME: This is a bit hacky. The only thing that we want to edit here is
	// to remove all inline keyboard, but flipWordCard in addition queries DB
	// for definition, which is unnecessary in this case.
	if err := flipWordCard(s.Clients, word, q.Message, nil); err != nil {
		return err
	}

	return practiceReply(s, chatID)
}

func answerIK(word string, ease AnswerEase, ivl int64) *InlineKeyboard {
	var text string
	switch ease {
	case AnswerAgain:
		text = "Again"
	case AnswerHard:
		text = "Hard"
	case AnswerGood:
		text = "Good"
	case AnswerEasy:
		text = "Easy"
	default:
		text = "UnknownEase"
	}
	if ivl > 0 {
		text = fmt.Sprintf("%s (%dd)", text, ivl)
	}
	return &InlineKeyboard{
		Text: text,
		CallbackData: CallbackInfo{
			Action: PracticeAnswerAction,
			Ease:   ease,
			Word:   word,
		}.String(),
	}
}

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
