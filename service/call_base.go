package service

import (
	"errors"
	"fmt"
	"math"
	"mkozhukh/chat/data"
	"time"

	gonanoid "github.com/matoous/go-nanoid"
)

type baseCallService struct {
	dao *data.DAO
	all *ServiceAll

	LivekitEnabled      bool
	notAcceptedTimeout  int // in seconds
	ReconnectingTimeout int // in seconds
}

func newCallService(dao *data.DAO, allService *ServiceAll, withLivekit bool) baseCallService {
	return baseCallService{
		dao:                 dao,
		all:                 allService,
		LivekitEnabled:      withLivekit,
		notAcceptedTimeout:  30,
		ReconnectingTimeout: 30,
	}
}

func (s *baseCallService) SendCallMessage(c *data.Call, messageType int) error {
	msg := &data.Message{
		Text:   "",
		ChatID: c.ChatID,
		UserID: c.InitiatorID,
		Type:   messageType,
	}
	return s.all.Informer.SendMessageEvent(c.ChatID, msg, "", 0, true)
}

func (s *baseCallService) CreateJoinToken(ctx *CallContext, callId int) (string, error) {
	if !s.LivekitEnabled {
		return "", data.ErrFeatureDisabled
	}

	call, err := s.dao.Calls.Get(callId)
	if err != nil {
		return "", err
	}

	err = s.checkUserAccess(call.ChatID, ctx.UserID)
	if err != nil {
		return "", err
	}

	token, err := s.all.Livekit.CreateJoinToken(call.RoomName, fmt.Sprintf("%d", ctx.UserID))

	return token, err
}

func (s *baseCallService) StartCallTimer(duration, id int, cb func(id int)) {
	time.AfterFunc(time.Duration(duration)*time.Second, func() {
		cb(id)
	})
}

func (s *baseCallService) precheckUserAccess(ctx *CallContext, targetChatId int, targetUserId int) (*data.Call, error) {
	err := s.checkUserAccess(targetChatId, ctx.UserID)
	if err != nil {
		return nil, err
	}

	// check if the current user is already in call
	check, err := s.dao.Calls.CheckIfUserInCall(ctx.UserID)
	if err != nil {
		return nil, err
	}
	if check {
		return nil, errors.New("already in the call")
	}

	if targetUserId != 0 {
		check, err := s.dao.Calls.CheckIfUserInCall(targetUserId)
		if err != nil {
			return nil, err
		}
		if check {
			call := data.Call{
				InitiatorID: ctx.UserID,
				Status:      data.CallStatusBusy,
				ChatID:      targetChatId,
			}
			s.SendCallMessage(&call, data.CallBusyMessage)
			return &call, nil
		}
	}

	return nil, nil
}

func (s *baseCallService) updateStatusAndSendMessage(call *data.Call, status int) error {
	var err error

	if status == data.CallStatusDisconnected {
		status = data.CallStatusEnded
	}

	if status > 900 {
		if call.Status > 900 {
			return nil
		}
		defer s.end(call)
	}
	if call.ChatID == 0 {
		return nil
	}

	if call.Status == data.CallStatusActive {
		status = data.CallStatusEnded
	}

	err = s.dao.Calls.Update(call, status)
	if err != nil {
		return err
	}

	msg := &data.Message{
		UserID: call.InitiatorID,
		ChatID: call.ChatID,
	}
	from := 0

	switch status {
	case data.CallStatusDisconnected, data.CallStatusEnded, data.CallStatusLost:
		var diff float64
		if call.Start != nil {
			diff = time.Since(*call.Start).Seconds()
			msg.Date = *call.Start
		}
		msg.Text = fmt.Sprintf("%02d:%02d", int(math.Floor(diff/60)), int(diff)%60)
		msg.Type = data.CallStartMessage
	case data.CallStatusRejected:
		msg.Type = data.CallRejectedMessage
	case data.CallStatusIgnored:
		msg.Type = data.CallMissedMessage
		from = -1
	default:
		msg = nil
	}

	if msg != nil {
		s.all.Informer.SendMessageEvent(call.ChatID, msg, "", from, true)
	}

	return nil
}

func (s *baseCallService) createRoom(c *data.Call) error {
	if !s.LivekitEnabled {
		return data.ErrFeatureDisabled
	}

	c.RoomName, _ = gonanoid.ID(16)
	err := s.dao.Calls.Save(c)
	if err != nil {
		return err
	}

	_, err = s.all.Livekit.CreateRoom(c.RoomName)
	if err != nil {
		c.Status = data.CallStatusLost
		s.dao.Calls.Save(c)
	}

	return err
}

func (s *baseCallService) checkUserAccess(chatId, userId int) error {
	chatusers, err := s.dao.UserChats.ByChat(chatId)
	if err != nil {
		return err
	}

	for _, u := range chatusers {
		if u.UserID == int(userId) {
			return nil
		}
	}

	return data.ErrAccessDenied
}

func (s *baseCallService) updateAcceptedCall(ctx *CallContext, call *data.Call) (bool, error) {
	cu := call.GetByUserID(ctx.UserID)
	if cu == nil {
		return false, data.ErrAccessDenied
	}

	if cu.Status == data.CallUserStatusActive {
		return false, fmt.Errorf("already in the call")
	}

	userStatus := data.CallUserStatusConnecting
	if cu.Status == data.CallUserStatusConnecting ||
		call.Status == data.CallStatusInitiated && cu.Status == data.CallUserStatusInitiated {
		userStatus = data.CallUserStatusActive
	}

	cu.Status = userStatus
	cu.DeviceID = ctx.DeviceID

	err := s.dao.CallUsers.UpdateUserDeviceID(call.ID, ctx.UserID, ctx.DeviceID, userStatus)

	return cu.Status == data.CallUserStatusActive, err
}

func (s *baseCallService) end(c *data.Call) error {
	if s.LivekitEnabled {
		// should delete the room as the call has been ended
		go s.all.Livekit.DeleteRoom(c.RoomName)
	}
	return s.dao.CallUsers.EndCall(c.ID)
}
