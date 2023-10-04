package service

import (
	"fmt"
	"mkozhukh/chat/data"
	"time"
)

type BotAPI struct {
	dao *data.DAO
}

type BotsConfig struct {
	OpenAI struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
		Proxy   string `json:"proxy"`
	} `json:"openai"`
}

type HistoryRecord struct {
	Author  int
	Message string
}

func (b *BotAPI) Error(err error, chatId int, userId int) {
	msg := data.Message{
		Text:   data.SafeHTML(err.Error()),
		ChatID: chatId,
		UserID: userId,
		Date:   time.Now(),
	}

	fmt.Printf("Error: %d %s\n", chatId, err)
	b.dao.Messages.SaveAndSend(chatId, &msg, "", userId)
}

func (b *BotAPI) Append(id int, text string, final bool, userId int) {
	b.dao.Messages.Append(id, text, final, userId)
}

func (b *BotAPI) Send(text string, chatId int, userId int) int {
	msg := data.Message{
		Text:   data.SafeHTML(text),
		Type:   data.BotMessage,
		ChatID: chatId,
		UserID: userId,
		Date:   time.Now(),
	}

	b.dao.Messages.SaveAndSend(chatId, &msg, "", 0)
	return msg.ID
}

func (b *BotAPI) GetHistory(chatId, count int) []data.Message {
	msgs, err := b.dao.Messages.GetLastN(chatId, count)
	if err != nil {
		return nil
	}

	return msgs
}

type Bot interface {
	Process(msg string, chanID int, api *BotAPI)
	GetID() int
}

type botsService struct {
	dao  *data.DAO
	bots map[int]Bot
	api  *BotAPI
}

func newBotsService(dao *data.DAO) *botsService {
	service := botsService{
		dao:  dao,
		bots: make(map[int]Bot),
		api: &BotAPI{
			dao: dao,
		},
	}

	return &service
}

func (s *botsService) Process(bot int, msg string, user, chat int) {
	b, ok := s.bots[bot]
	if !ok {
		return
	}

	go b.Process(msg, chat, s.api)
}

func (s *botsService) IsBot(user int) bool {
	_, ok := s.bots[user]
	return ok
}

func (s *botsService) AddBot(bot Bot) {
	s.bots[bot.GetID()] = bot
}
