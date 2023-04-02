package api

import (
	"mkozhukh/chat/data"
	"mkozhukh/chat/service"

	remote "github.com/mkozhukh/go-remote"
)

type ChatsAPI struct {
	db   *data.DAO
	sAll *service.ServiceAll
}

type ChatEvent struct {
	Op     string                `json:"op"`
	UserId int                   `json:"user_id"`
	ChatID int                   `json:"chat_id"`
	Users  []int                 `json:"-"`
	Data   *data.UserChatDetails `json:"data"`
}

func (d *ChatsAPI) AddDirect(targetUserId int, userId UserID, events *remote.Hub) (*data.UserChatDetails, error) {
	chatId, err := d.db.Chats.AddDirect(targetUserId, int(userId))
	if err != nil {
		return nil, err
	}

	info, err := d.db.UserChats.GetOne(chatId, int(userId))
	if err != nil {
		return nil, err
	}

	// message sent for other user, so change DirectID accordingly
	messageInfo := *info
	messageInfo.DirectID = int(userId)
	events.Publish("chats", ChatEvent{Op: "add", ChatID: chatId, Data: &messageInfo, UserId: int(userId)})

	return info, nil
}

func (d *ChatsAPI) AddGroup(name, avatar string, users []int, userId UserID, events *remote.Hub) (*data.UserChatDetails, error) {
	// sanitize input
	name = safeHTML(name)
	avatar = safeUrl(avatar)

	chatId, err := d.db.Chats.AddGroup(name, avatar, append(users, int(userId)))
	if err != nil {
		return nil, err
	}

	info, err := d.db.UserChats.GetOne(chatId, int(userId))
	if err != nil {
		return nil, err
	}

	events.Publish("chats", ChatEvent{Op: "add", ChatID: chatId, Data: info, UserId: int(userId)})

	return info, nil
}

func (d *ChatsAPI) Update(chatId int, name string, avatar string, userId UserID, events *remote.Hub) (*data.UserChatDetails, error) {
	if !d.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, data.ErrAccessDenied
	}

	// sanitize input
	name = safeHTML(name)
	avatar = safeUrl(avatar)

	err := d.db.Chats.Update(chatId, name, avatar)
	if err != nil {
		return nil, err
	}

	return d.getChatInfo(chatId, int(userId), events, nil)
}

func (d *ChatsAPI) SetUsers(chatId int, users []int, userId UserID, events *remote.Hub) (*data.UserChatDetails, error) {
	if !d.db.UsersCache.HasChat(int(userId), chatId) {
		return nil, data.ErrAccessDenied
	}

	oldUsers := d.db.UsersCache.GetUsers(chatId)
	updUsers := append(users, int(userId))
	chatId, err := d.db.Chats.SetUsers(chatId, updUsers)
	if err != nil {
		return nil, err
	}

	// update call users
	err = d.sAll.GroupCalls.RefreshCallUsers(chatId, updUsers)
	if err != nil {
		return nil, err
	}

	return d.getChatInfo(chatId, int(userId), events, oldUsers)
}

func (d *ChatsAPI) Leave(chatId int, userId UserID, events *remote.Hub) error {
	if !d.db.UsersCache.HasChat(int(userId), chatId) {
		return data.ErrAccessDenied
	}

	oldUsers := d.db.UsersCache.GetUsers(chatId)

	err := d.db.Chats.Leave(chatId, int(userId))
	if err != nil {
		return err
	}

	if data.Features.WithGroupCalls {
		call, err := d.db.Calls.CheckIfChatInCall(chatId)
		if err != nil {
			return err
		}
		// notify to disconnect user
		d.sAll.Informer.SendSignalToCall(
			&call,
			data.CallStatusDisconnected,
			data.CallUser{UserID: int(userId)},
		)
	}

	info, err := d.db.UserChats.GetOneLeaved(chatId)
	if err != nil {
		return err
	}

	d.sendChatInfo(chatId, int(userId), info, events, oldUsers)
	return nil
}

func (d *ChatsAPI) getChatInfo(chatId, userId int, events *remote.Hub, targetUsers []int) (*data.UserChatDetails, error) {
	info, err := d.db.UserChats.GetOne(chatId, int(userId))
	if err != nil {
		return nil, err
	}

	d.sendChatInfo(chatId, userId, info, events, targetUsers)
	return info, nil
}

func (d *ChatsAPI) sendChatInfo(chatId, userId int, info *data.UserChatDetails, events *remote.Hub, targetUsers []int) {
	// prevent leaking of personal info
	einfo := *info
	einfo.DirectID = 0
	einfo.UnreadCount = 0
	einfo.Status = 0
	events.Publish("chats", ChatEvent{Op: "update", ChatID: chatId, Data: &einfo, UserId: userId, Users: targetUsers})
}
