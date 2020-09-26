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
// actions should contain action implementation for different actions that
// users can take.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
)

// Questions:
// * Should actions be exclusive of one another? If one is executed, the rest are skipped?
// * What about calling if !Match in each action to make sure preconditions hold? NO. Difficult chaining to reuse.
type Action interface {
	// Perform the action.
	Perform(*Update) ([]Action, error)
	// Whether this action should be performed as a result of this update.
	Match(*Update) bool
}

// TODO: I am not sure if this is the best decision to bundle all up together.
// All objects needed to perform actions.
type Clients struct {
	Telegram    *Telegram
	Definer     *Definer
	Repetitions *Repetition
	Settings    *SettingsConfig
}

// Actions that always can be executed independent of other things.
func BaseActions(c *Clients) []Action {
	return []Action{
		&DontKnowAction{c, ""},
		&LearnAction{c},
		&DeleteWordAction{c},
		&CatchAllAction{c, ""},
	}
}

// actions shared by all commands:
// b:Don't know action (reset) - to work with all messages, no further
//    practice to display.
// b:Learn - should always be possible to add another word to the ones learning

// THese are more for RegularActions or Default actions
func DefaultActions(c *Clients) []Action {
	return []Action{
		&StartAction{c},
		&SettingsAction{c},
		&PracticeAction{c},
		&AddCommandAction{c},
		&DefineWordAction{c},
		// /start
		// /settings
		// /practice
		// word
		/// catch the rest and show help?
	}
}

// Practice Actions:
// b:Know & display another (If exited practice mode, or word from callback !=
//                           word from action - do nothing)
// b:Don't know & display another (same as b:Know)
// /stop
// /settings
/// <catch the rest and show help>
// (may or may not include /start)

// func NewPracticeActions(c *Clients, asking string) []Action {
// 	return []Action{
// 		&StartAction{},
// 	}
// }

// PracticeKnowAction{asking string}
// PracticeKnowAction.Perform{ ... return NewPracticeState(a.asking) }

type StartAction struct {
	*Clients
}

// CallbackAction as a special action to handle all callbacks?

func (a *StartAction) Perform(u *Update) ([]Action, error) {
	// It might be beneficial not to reset actions here.
	return DefaultActions(a.Clients), a.Telegram.SendTextMessage(u.Message.Chat.Id,
		"Welcome to the language bot. Still in development. No instructions "+
			"so far. "+
			"All sentences and translations are from Tatoeba's (https://tatoeba.org) "+
			"dataset, released under a CC-BY 2.0 FR.")
}

func (*StartAction) Match(u *Update) bool {
	// FIXME: This looks refactorable!
	if u.Message == nil {
		return false
	}
	return u.Message.Text == "/start"
}

type StopAction struct {
	*Clients
	Message string
}

func (a *StopAction) Perform(u *Update) ([]Action, error) {
	msg := a.Message
	if msg == "" {
		msg = "Stopped."
	}
	return DefaultActions(a.Clients), a.Telegram.SendTextMessage(u.Message.Chat.Id,
		msg)
}

func (a *StopAction) Match(u *Update) bool {
	if u.Message == nil {
		return false
	}
	return u.Message.Text == "/stop"
}

// An action to be executed if no other action was matched.
type CatchAllAction struct {
	*Clients
	Message string
}

func (a *CatchAllAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	msg := a.Message
	if msg == "" {
		switch {
		case u.Message != nil:
			msg += fmt.Sprintf("Couldn't process your message %q", u.Message.Text)
		case u.CallbackQuery != nil:
			msg += fmt.Sprintf("INTERNAL: Due to restart or a bug couldn't process callback query %v", u.CallbackQuery)
		default:
			msg += fmt.Sprintf("INTERNAL: Cannot handle update %v", u)
		}
	}
	return nil, a.Telegram.SendTextMessage(chatId, msg)
}

func (a *CatchAllAction) Match(u *Update) bool {
	return true
}

type CatchAllMessagesAction struct{ CatchAllAction }

func (a *CatchAllMessagesAction) Match(u *Update) bool {
	return u.Message != nil
}

// FIXME: Should they be "inherited" from Clients? (I think that makes sense)
type SettingsAction struct {
	*Clients
}

func (a *SettingsAction) Match(u *Update) bool {
	if u.Message == nil {
		return false
	}
	return u.Message.Text == "/settings"
}

func (a *SettingsAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	s, err := a.Settings.Get(chatId)
	if err != nil {
		return nil, err
	}
	var ls []string
	for l, v := range s.TranslationLanguages {
		if v {
			ls = append(ls, fmt.Sprintf("%q", l))
		}
	}
	sort.Strings(ls)
	msg := fmt.Sprintf(`
Current settings:

Input language: %q
Input language in ISO 639-3: %q
Translation languages in ISO 639-3: %s
Time Zone: %s

Choose setting which you want to modify.
(The choice might improve in the future.)
`, s.InputLanguage, s.InputLanguageISO639_3, strings.Join(ls, ","), s.TimeZone)
	return []Action{
			// Note that stop should be handled before input language is!
			&StopAction{a.Clients, "Exited settings"},
			&InputLanguageButton{a.Clients},
			&TimeZoneButton{a.Clients},
			&PracticeAction{a.Clients},
			&CatchAllMessagesAction{CatchAllAction{
				a.Clients, "type /stop to exit settings"}},
		}, a.Telegram.SendMessage(&MessageReply{
			ChatId: chatId,
			Text:   msg,
			ReplyMarkup: &ReplyMarkup{
				InlineKeyboard: [][]*InlineKeyboard{{
					{
						Text: "Input Language",
						CallbackData: CallbackInfo{
							Action:  ChangeSettingAction,
							Setting: "InputLanguage",
						}.String(),
					},
					{
						Text: "Time Zone",
						CallbackData: CallbackInfo{
							Action:  ChangeSettingAction,
							Setting: "TimeZone",
						}.String(),
					},
				}},
			},
		})
}

type InputLanguageButton struct {
	*Clients
}

func (b *InputLanguageButton) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	var ls []string
	for l, _ := range SupportedInputLanguages {
		ls = append(ls, fmt.Sprintf("%q", l))
	}
	sort.Strings(ls)
	return []Action{
			&StopAction{b.Clients, "Exited settings"},
			&PickLanguageAction{b.Clients},
		}, b.Telegram.SendTextMessage(
			chatId,
			fmt.Sprintf("Enter input language of your choice. Supported are %s",
				strings.Join(ls, ",")))
}

func (b *InputLanguageButton) Match(u *Update) bool {
	if u.CallbackQuery == nil {
		return false
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	return info.Setting == "InputLanguage"
}

type TimeZoneButton struct {
	*Clients
}

func (b *TimeZoneButton) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	return []Action{
			&StopAction{b.Clients, "Exited settings"},
			&PickTimeZoneAction{b.Clients},
		}, b.Telegram.SendMessage(&MessageReply{
			ChatId: chatId,
			Text:   "Input your timezone in format (UTC, UTC+X or UTC-X)",
		})
}

func (b *TimeZoneButton) Match(u *Update) bool {
	if u.CallbackQuery == nil {
		return false
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	return info.Setting == "TimeZone"
}

type PickLanguageAction struct {
	*Clients
}

func (a *PickLanguageAction) Perform(u *Update) ([]Action, error) {
	m := u.Message
	chatId := m.Chat.Id
	err := a.Settings.SetLanguage(chatId, m.Text)
	if err != nil {
		return nil, a.Telegram.SendTextMessage(chatId, "Unsupported language. Try again.")
	}
	sa := SettingsAction{a.Clients}
	return sa.Perform(u)
}

func (a *PickLanguageAction) Match(u *Update) bool {
	return u.Message != nil
}

type PickTimeZoneAction struct {
	*Clients
}

func (a *PickTimeZoneAction) Perform(u *Update) ([]Action, error) {
	m := u.Message
	chatId := m.Chat.Id
	set := TimeZones[m.Text]
	if !set {
		return nil, a.Telegram.SendTextMessage(chatId, "Unsupported time zone. Try again (format should be UTC, UTC+X or UTC-X).")
	}
	a.Settings.SetTimeZone(chatId, m.Text)
	sa := SettingsAction{a.Clients}
	return sa.Perform(u)
}

func (a *PickTimeZoneAction) Match(u *Update) bool {
	return u.Message != nil
}

type PracticeAction struct {
	*Clients
}

func (a *PracticeAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	switch UsePractice {
	case PracticeWordSpelling:
		// FIXME: This is not tested!!!
		d, err := a.Repetitions.Repeat(chatId)
		if err == sql.ErrNoRows {
			// FIXME: This might be refactorable
			return DefaultActions(a.Clients),
				a.Telegram.SendTextMessage(chatId, "No more rows to practice; exiting practice mode.")
		}
		if err != nil {
			return nil, fmt.Errorf("retrieving word for repetition: %w", err)
		}
		return []Action{
			&StopAction{a.Clients, "Stopped practice"},
			&SettingsAction{a.Clients},
			&AnswerAction{a.Clients, d},
			// FIXME: Obfuscate definition before sending it to the user!!!
		}, a.Telegram.SendTextMessage(chatId, d)
	case PracticeKnowledge:
		w, err := a.Repetitions.RepeatWord(chatId)
		if err == sql.ErrNoRows {
			return DefaultActions(a.Clients),
				a.Telegram.SendTextMessage(chatId, "No more rows to practice; exiting practice mode.")
		}
		if err != nil {
			return nil, fmt.Errorf("retrieving word for repetition: %w", err)
		}
		ka := &KnowAction{a.Clients, w}
		dka := &DontKnowAction{a.Clients, w}
		return []Action{
				&StopAction{a.Clients, "Stopped practice"},
				&SettingsAction{a.Clients},
				ka,
				dka,
				&CatchAllMessagesAction{CatchAllAction{
					a.Clients, "type /stop to exit practice mode"}},
			}, a.Telegram.SendMessage(&MessageReply{
				ChatId: chatId,
				Text:   w,
				ReplyMarkup: &ReplyMarkup{
					InlineKeyboard: [][]*InlineKeyboard{[]*InlineKeyboard{
						ka.AsKeyboard(),
						dka.AsKeyboard(),
					}},
				},
			})
	default:
		return nil, fmt.Errorf("INTERNAL: Unimplemented practice type: %v", UsePractice)
	}
}

func (a *PracticeAction) Match(u *Update) bool {
	if u.Message == nil {
		return false
	}
	return u.Message.Text == "/practice"
}

// TODO: Can I not extract word from the message? m.Text?
func flipWordCard(c *Clients, word string, m *Message, ks []*InlineKeyboard) error {
	// TODO: It isn't always neccessary to retrieve defitnion when this
	// function is used.
	def, err := c.Repetitions.GetDefinition(m.Chat.Id, word)
	if err != nil {
		return fmt.Errorf("retrieving definition: %v", err)
	}
	if ks == nil {
		ks = []*InlineKeyboard{}
	}
	r := &EditMessageText{
		ChatId:    m.Chat.Id,
		MessageId: m.Id,
		// TODO: Enable replying in markdown, but for that need to store
		// definitions escaped.
		//ParseMode:   "MarkdownV2",
		Text: def,
		// FIXME: Should InlineKeyboard be refactored for less duplication?
		ReplyMarkup: ReplyMarkup{
			InlineKeyboard: [][]*InlineKeyboard{ks},
		},
	}
	var rm Message
	if err := c.Telegram.Call("editMessageText", r, &rm); err != nil {
		return fmt.Errorf("editing message: %w", err)
	}
	return nil
}

type KnowAction struct {
	*Clients
	Word string
}

func (a *KnowAction) Perform(u *Update) ([]Action, error) {
	defer a.Telegram.AnswerCallbackLog(u.CallbackQuery.Id, "")
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	word := info.Word
	if err := a.Repetitions.AnswerKnow(chatId, word); err != nil {
		return nil, err
	}
	dka := &DontKnowAction{a.Clients, a.Word}
	if err := flipWordCard(a.Clients, word, u.CallbackQuery.Message, []*InlineKeyboard{dka.AsKeyboard()}); err != nil {
		return nil, err
	}
	pa := PracticeAction{a.Clients}
	return pa.Perform(u)
}

func (a *KnowAction) Match(u *Update) bool {
	if u.CallbackQuery == nil {
		return false
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	return info.Action == PracticeKnowAction && info.Word == a.Word
}

func (a *KnowAction) AsKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Know",
		CallbackData: CallbackInfo{
			Action: PracticeKnowAction,
			Word:   a.Word,
		}.String(),
	}
}

type DontKnowAction struct {
	*Clients
	Word string
}

func (a *DontKnowAction) Perform(u *Update) ([]Action, error) {
	defer a.Telegram.AnswerCallbackLog(u.CallbackQuery.Id, "Reset progress")
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	word := info.Word
	if err := a.Repetitions.AnswerDontKnow(chatId, word); err != nil {
		return nil, err
	}
	if err := flipWordCard(a.Clients, word, u.CallbackQuery.Message, nil); err != nil {
		return nil, err
	}
	// Clicking Don't know on the previous messages shouldn't result in
	// practice being continued.
	if word != a.Word {
		return nil, nil
	}
	pa := PracticeAction{a.Clients}
	return pa.Perform(u)
}

func (a *DontKnowAction) Match(u *Update) bool {
	if u.CallbackQuery == nil {
		return false
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	return info.Action == PracticeDontKnowAction
}

func (a *DontKnowAction) AsKeyboard() *InlineKeyboard {
	return &InlineKeyboard{
		Text: "Don't know",
		CallbackData: CallbackInfo{
			Action: PracticeDontKnowAction,
			Word:   a.Word,
		}.String(),
	}
}

type AnswerAction struct {
	*Clients
	Question string
}

func (a *AnswerAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	m := u.Message
	correct, err := a.Repetitions.Answer(chatId, a.Question, m.Text)
	if err != nil {
		return nil, err
	}
	if correct != m.Text {
		return nil, a.Telegram.SendTextMessage(chatId, fmt.Sprintf("Correct word: %q", correct))
	}
	// correct == m.Text
	congrats := [...]string{"Good job!", "Well done!", "Awesome!", "Keep it up!", "Excellent!", "You've got it!", "Great!", "Terrific!", "Ваще красава!11"}
	if err := a.Telegram.SendTextMessage(chatId, congrats[rand.Int()%len(congrats)]); err != nil {
		return nil, err
	}
	pa := PracticeAction{a.Clients}
	return pa.Perform(u)
}

func (a *AnswerAction) Match(u *Update) bool {
	return u.Message != nil
}

type AddCommandAction struct {
	*Clients
}

func (a *AddCommandAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	return []Action{
			&StopAction{a.Clients, "Cancelled addition"},
			&SettingsAction{a.Clients},
			&AddWordAction{a.Clients},
		}, a.Telegram.SendTextMessage(chatId, `
Enter the card you want to add in the format:
<front of the card (word, expression, question)>
<back of the card (can be multiline)>
`)
}

func (a *AddCommandAction) Match(u *Update) bool {
	return u.Message != nil && u.Message.Text == "/add"
}

type AddWordAction struct {
	*Clients
}

func (a *AddWordAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	parts := strings.Split(u.Message.Text, "\n")
	if len(parts) < 2 {
		return nil, a.Telegram.SendTextMessage(chatId, "Wrong format")
	}
	front := parts[0]
	back := strings.Join(parts[1:], "\n")
	if err := a.Repetitions.Save(chatId, front, back); err != nil {
		return nil, err
	}
	return DefaultActions(a.Clients), a.Telegram.SendTextMessage(chatId, fmt.Sprintf("Added %q for learning!", front))
}

func (a *AddWordAction) Match(u *Update) bool {
	return u.Message != nil
}

type DefineWordAction struct {
	*Clients
}

func (a *DefineWordAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	m := u.Message
	if len(strings.Split(m.Text, " ")) > 1 {
		return nil, a.Telegram.SendTextMessage(chatId, "INTERNAL ERROR: DefineWordAction should not have been called with many words. Is Match called correctly?")
	}

	def, err := a.Repetitions.GetDefinition(m.Chat.Id, m.Text)
	if err == nil {
		dka := &DontKnowAction{a.Clients, m.Text}
		k := dka.AsKeyboard()
		k.Text = "Reset progress"
		return nil, a.Telegram.SendMessage(&MessageReply{
			ChatId: m.Chat.Id,
			Text:   def,
			ReplyMarkup: &ReplyMarkup{
				InlineKeyboard: [][]*InlineKeyboard{
					[]*InlineKeyboard{k},
				},
			},
		})
	}
	if err != sql.ErrNoRows {
		log.Printf("ERROR: Repetitions(%d, %s): %v", m.Chat.Id, m.Text, err)
	}
	settings, err := a.Settings.Get(chatId)
	if err != nil {
		return nil, fmt.Errorf("get settings: %v", err)
	}
	ds, err := a.Definer.Define(m.Text, settings)
	if err != nil {
		// TODO: Might be good to post debug logs to the reply in the debug mode.
		log.Printf("Error fetching the definition: %w", err)
		// FIXME: Add search url to the reply?
		return nil, a.Telegram.SendTextMessage(chatId, "Couldn't find definitions")
	}

	for _, d := range ds {
		if err := a.Telegram.SendMessage(&MessageReply{
			ChatId:    m.Chat.Id,
			Text:      d,
			ParseMode: "MarkdownV2",
			ReplyMarkup: &ReplyMarkup{
				InlineKeyboard: [][]*InlineKeyboard{[]*InlineKeyboard{
					&InlineKeyboard{
						Text: "Learn",
						CallbackData: CallbackInfo{
							Action: SaveWordAction,
							Word:   m.Text,
						}.String(),
					},
				}},
			},
		}); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (a *DefineWordAction) Match(u *Update) bool {
	if u.Message == nil {
		return false
	}
	return len(strings.Split(u.Message.Text, " ")) == 1
}

type LearnAction struct {
	*Clients
}

func (a *LearnAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	word := info.Word
	if err := a.Repetitions.Save(chatId, word, u.CallbackQuery.Message.Text); err != nil {
		return nil, err
	}
	m := u.CallbackQuery.Message
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
	if err := a.Telegram.Call("editMessageReplyMarkup", r, &rm); err != nil {
		return nil, fmt.Errorf("editing message reply markup: %w", err)
	}
	msg := fmt.Sprintf("Saved %q for learning", word)
	a.Telegram.AnswerCallbackLog(u.CallbackQuery.Id, msg)
	return nil, nil
}

func (a *LearnAction) Match(u *Update) bool {
	if u.CallbackQuery == nil {
		return false
	}
	info := CallbackInfoFromString(u.CallbackQuery.Data)
	return info.Action == SaveWordAction
}

// DeleteWordAction stops learning of the word.
type DeleteWordAction struct {
	*Clients
}

func (a *DeleteWordAction) Match(u *Update) bool {
	if u.Message == nil {
		return false
	}
	return strings.HasPrefix(u.Message.Text, "/delete")
}

func (a *DeleteWordAction) Perform(u *Update) ([]Action, error) {
	chatId, err := u.ChatId()
	if err != nil {
		return nil, err
	}
	word := strings.TrimSpace(strings.TrimPrefix(u.Message.Text, "/delete"))
	e, err := a.Repetitions.Exists(chatId, word)
	if err != nil {
		return nil, err
	}
	if !e {
		return nil, a.Telegram.SendTextMessage(chatId, fmt.Sprintf("Word %q isn't saved for learning!", word))
	}
	if err := a.Repetitions.Delete(chatId, word); err != nil {
		return nil, err
	}
	return nil, a.Telegram.SendTextMessage(chatId, fmt.Sprintf("Deleted %q!", word))
}
