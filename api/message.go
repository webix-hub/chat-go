package api

import (
	"mkozhukh/chat/data"
	"time"

	remote "github.com/mkozhukh/go-remote"
)

type MessagesAPI struct {
	db *data.DAO
}

type MessageEvent struct {
	Op     string        `json:"op"`
	Msg    *data.Message `json:"msg"`
	Origin string        `json:"origin,omitempty"`
}

func (m *MessagesAPI) GetAll(chatId int, userId UserID) ([]data.Message, error) {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, AccessDeniedError
	}

	return m.db.Messages.GetAll(chatId)
}

func (m *MessagesAPI) ResetCounter(chatId int, userId UserID) error {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return AccessDeniedError
	}

	err := m.db.UserChats.ResetCounter(chatId, int(userId))
	if err != nil {
		return err
	}

	return nil
}

func (m *MessagesAPI) Add(text string, chatId int, origin string, userId UserID, events *remote.Hub) (*data.Message, error) {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, AccessDeniedError
	}

	msg := data.Message{
		Text:   safeHTML(text),
		ChatID: chatId,
		UserID: int(userId),
		Date:   time.Now(),
	}

	err := m.db.Messages.Save(&msg)
	if err != nil {
		return nil, err
	}

	events.Publish("messages", MessageEvent{Op: "add", Msg: &msg, Origin: origin})

	err = m.db.UserChats.IncrementCounter(chatId, int(userId))
	if err != nil {
		return nil, err
	}

	_, err = m.db.Chats.SetLastMessage(chatId, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func (m *MessagesAPI) Update(msgID int, text string, userId UserID, events *remote.Hub) (*data.Message, error) {
	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return nil, err
	}

	if msg.UserID != int(userId) || !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return nil, AccessDeniedError
	}

	msg.Text = safeHTML(text)

	ch, err := m.db.Chats.GetOne(msg.ChatID)
	if err != nil {
		return nil, err
	}

	err = m.db.Messages.Save(msg)
	if err != nil {
		return nil, err
	}

	events.Publish("messages", MessageEvent{Op: "update", Msg: msg})
	if ch.LastMessage == msg.ID {
		events.Publish("chats", ChatEvent{Op: "message", ChatID: msg.ChatID, Data: &data.UserChatDetails{Message: msg.Text, Date: &msg.Date}, UserId: 0})
	}

	// [FIXME] we must not increment non-zero counters
	err = m.db.UserChats.IncrementCounter(msg.ChatID, int(userId))
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (m *MessagesAPI) Remove(msgID int, userId UserID, events *remote.Hub) error {
	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return err
	}

	if msg.UserID != int(userId) || !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return AccessDeniedError
	}

	err = m.db.Messages.Delete(msgID)
	if err != nil {
		return err
	}

	ch, err := m.db.Chats.GetOne(msg.ChatID)
	if err != nil {
		return err
	}

	events.Publish("messages", MessageEvent{Op: "remove", Msg: &data.Message{ID: msgID, ChatID: msg.ChatID}})
	if ch.LastMessage == msg.ID {
		msg, err = m.db.Chats.SetLastMessage(msg.ChatID, nil)
		events.Publish("chats", ChatEvent{Op: "message", ChatID: msg.ChatID, Data: &data.UserChatDetails{Message: msg.Text, Date: &msg.Date}, UserId: 0})
	}

	return nil
}
