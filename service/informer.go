package service

import (
	"encoding/json"
	"mkozhukh/chat/data"

	remote "github.com/mkozhukh/go-remote"
)

type CallDevices struct {
	Devices []int `json:"devices"`
}

type Signal struct {
	Message string `json:"msg"`
	Type    string `json:"type"`
	Users   []int  `json:"-"`
	Devices []int  `json:"-"`
}

type informerService struct {
	dao *data.DAO
	hub *remote.Hub
}

func newInformerService(dao *data.DAO, hub *remote.Hub) *informerService {
	return &informerService{
		dao: dao,
		hub: hub,
	}
}

func (s *informerService) SendSignal(kind string, payload interface{}, users, devices []int) {
	var msg string
	if s, ok := payload.(string); ok {
		msg = s
	} else {
		bytes, _ := json.Marshal(&payload)
		msg = string(bytes)
	}

	s.hub.Publish("signal", Signal{
		Type:    kind,
		Message: msg,
		Users:   users,
		Devices: devices,
	})
}

func (s *informerService) SendSignalToCall(c *data.Call, status int, to ...data.CallUser) {
	if status == 0 {
		status = c.Status
	}

	msgData := Call{
		ID:          c.ID,
		Status:      status,
		Start:       c.Start,
		InitiatorID: c.InitiatorID,
		IsGroupCall: c.IsGroupCall,
		ChatID:      c.ChatID,
		Users:       c.GetUsersIDs(false),
	}

	var devices []int
	var users []int
	if to != nil {
		for _, cu := range to {
			devices = append(devices, cu.DeviceID)
			users = append(users, cu.UserID)
		}
	} else {
		devices = c.GetDevicesIDs(true)
		users = c.GetUsersIDs(true)
	}

	s.SendSignal("connect", msgData, users, devices)
}

func (s *informerService) SendSignalToUser(targetUserId int, payload interface{}) {
	s.SendSignal("connect", payload, []int{targetUserId}, []int{0})
}

func (s *informerService) SendMessageEvent(chatId int, msg *data.Message, origin string, from int, save bool) error {
	if save {
		err := s.dao.Messages.Save(msg)
		if err != nil {
			return err
		}
	}
	s.hub.Publish("messages", data.MessageEvent{Op: "add", Msg: msg, Origin: origin, From: from})

	err := s.dao.UserChats.IncrementCounter(chatId, msg.UserID)
	if err == nil {
		_, err = s.dao.Chats.SetLastMessage(chatId, msg)
	}

	return err
}
