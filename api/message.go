package api

import (
	"errors"
	"mkozhukh/chat/data"
	"mkozhukh/chat/service"
	"time"

	remote "github.com/mkozhukh/go-remote"
)

type MessagesAPI struct {
	db     *data.DAO
	sAll   *service.ServiceAll
	config data.FeaturesConfig
}

func (m *MessagesAPI) GetAll(chatId int, userId UserID) ([]data.Message, error) {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, data.ErrAccessDenied
	}

	return m.db.Messages.GetAll(chatId)
}

func (m *MessagesAPI) ResetCounter(chatId int, userId UserID) error {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return data.ErrAccessDenied
	}

	err := m.db.UserChats.ResetCounter(chatId, int(userId))
	if err != nil {
		return err
	}

	return nil
}

func (m *MessagesAPI) Add(text string, chatId int, origin string, userId UserID, deviceId DeviceID, events *remote.Hub) (*data.Message, error) {
	if !m.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, data.ErrAccessDenied
	}

	msg := data.Message{
		Text:   data.SafeHTML(text),
		ChatID: chatId,
		UserID: int(userId),
		Date:   time.Now(),
	}

	err := m.db.Messages.SaveAndSend(chatId, &msg, origin, int(deviceId))
	if err != nil {
		return nil, err
	}

	if m.config.WithBots {
		users := m.db.UsersCache.GetUsers(chatId)
		for _, u := range users {
			if m.sAll.Bots.IsBot(u) {
				go m.sAll.Bots.Process(u, msg.Text, int(userId), chatId)
			}
		}
	}

	return &msg, nil
}

func (m *MessagesAPI) Update(msgID int, text string, userId UserID, deviceId DeviceID, events *remote.Hub) (*data.Message, error) {
	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return nil, err
	}

	if msg.UserID != int(userId) || !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return nil, data.ErrAccessDenied
	}

	msg.Text = data.SafeHTML(text)
	msg.Edited = true

	ch, err := m.db.Chats.GetOne(msg.ChatID)
	if err != nil {
		return nil, err
	}

	err = m.db.Messages.Save(msg)
	if err != nil {
		return nil, err
	}

	events.Publish("messages", data.MessageEvent{Op: "update", Msg: msg, From: int(deviceId)})
	if ch.LastMessage == msg.ID {
		events.Publish("chats", ChatEvent{Op: "message", ChatID: msg.ChatID, Data: &data.UserChatDetails{Message: msg.Text, MessageType: msg.Type, Date: &msg.Date}, UserId: 0})
	}

	// [FIXME] we must not increment non-zero counters
	err = m.db.UserChats.IncrementCounter(msg.ChatID, int(userId))
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (m *MessagesAPI) Remove(msgID int, userId UserID, deviceId DeviceID, events *remote.Hub) error {
	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return err
	}

	if msg.UserID != int(userId) || !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return data.ErrAccessDenied
	}

	err = m.db.Messages.Delete(msgID)
	if err != nil {
		return err
	}

	ch, err := m.db.Chats.GetOne(msg.ChatID)
	if err != nil {
		return err
	}

	events.Publish(
		"messages",
		data.MessageEvent{Op: "remove", Msg: &data.Message{ID: msgID, ChatID: msg.ChatID}, From: int(deviceId)},
	)

	if ch.LastMessage == msg.ID {
		msg, err = m.db.Chats.SetLastMessage(msg.ChatID, nil)
		if err != nil {
			return err
		}
		events.Publish("chats", ChatEvent{Op: "message", ChatID: msg.ChatID, Data: &data.UserChatDetails{Message: msg.Text, MessageType: msg.Type, Date: &msg.Date}, UserId: 0})
	}

	return nil
}

func (m *MessagesAPI) AddReaction(msgID int, reaction string, userId UserID, deviceId DeviceID, events *remote.Hub) (*data.Message, error) {
	if !m.config.WithReactions {
		return nil, data.ErrFeatureDisabled
	}

	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return nil, err
	}
	if msg.UserID == int(userId) {
		return nil, errors.New("cannot add a reaction to own message")
	}
	if !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return nil, data.ErrAccessDenied
	}

	v := data.Reaction{
		MessageId: msgID,
		Reaction:  reaction,
		UserId:    int(userId),
	}
	added, err := m.db.Reactions.Add(v)
	if err != nil || !added {
		return nil, err
	}

	msg.Reactions[reaction] = append(msg.Reactions[reaction], v.UserId)
	events.Publish("messages", data.MessageEvent{Op: "update", Msg: msg, From: int(deviceId)})

	return msg, nil
}

func (m *MessagesAPI) RemoveReaction(msgID int, reaction string, userId UserID, deviceId DeviceID, events *remote.Hub) (*data.Message, error) {
	if !m.config.WithReactions {
		return nil, data.ErrFeatureDisabled
	}

	msg, err := m.db.Messages.GetOne(msgID)
	if err != nil {
		return nil, err
	}
	if !m.db.UsersCache.HasChat(int(userId), msg.ChatID) {
		return nil, data.ErrAccessDenied
	}

	r := data.Reaction{
		MessageId: msgID,
		Reaction:  reaction,
		UserId:    int(userId),
	}
	err = m.db.Reactions.Remove(r)
	if err != nil {
		return nil, err
	}

	if len(msg.Reactions[reaction]) <= 1 {
		delete(msg.Reactions, reaction)
	} else {
		for i, id := range msg.Reactions[reaction] {
			if id == r.UserId {
				msg.Reactions[reaction] = append(
					msg.Reactions[reaction][:i],
					msg.Reactions[reaction][i+1:]...,
				)
				break
			}
		}
	}

	events.Publish("messages", data.MessageEvent{Op: "update", Msg: msg, From: int(deviceId)})

	return msg, err
}
