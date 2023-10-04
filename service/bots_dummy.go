package service

import (
	"strconv"
	"time"
)

type DummyLengthBot struct {
	ID int
}

func (b *DummyLengthBot) Process(msg string, chatId int, api *BotAPI) {
	time.Sleep(100 * time.Millisecond)
	api.Send("Message length: "+strconv.Itoa(len(msg)), chatId, b.GetID())
}

func (b *DummyLengthBot) GetID() int {
	return b.ID
}
