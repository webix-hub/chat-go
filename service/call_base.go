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

	reconnectingUsers map[int]int64
}

func newCallService(dao *data.DAO, allService *ServiceAll, withLivekit bool) baseCallService {
	return baseCallService{
		dao:                 dao,
		all:                 allService,
		LivekitEnabled:      withLivekit,
		notAcceptedTimeout:  30,
		ReconnectingTimeout: 30,
		reconnectingUsers:   make(map[int]int64),
	}
}

func (s *baseCallService) StartReconnectingTimer(ctx *CallContext, callId int) {
	reconnectingTime := time.Now().Unix()
	s.reconnectingUsers[ctx.UserID] = reconnectingTime

	s.StartCallTimer(s.ReconnectingTimeout, callId, func(_ int) {
		if t, ok := s.reconnectingUsers[ctx.UserID]; ok && t != reconnectingTime {
			return
		}

		call, err := s.dao.Calls.Get(callId)
		if err != nil {
			return
		}

		if call.Status > 900 {
			return
		}

		u := call.GetByUserID(ctx.UserID)
		if u != nil && u.Status == data.CallUserStatusConnecting {
			// drop call if the user's reconnecting timed out
			callService := CallProvider.GetService(call.IsGroupCall)
			callService.Disconnect(ctx, &call, data.CallStatusDisconnected)
		}
	})
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

func (s *baseCallService) DropAllCalls(status int) error {
	calls, err := s.dao.Calls.DropAllCalls(status)
	if err != nil {
		return err
	}

	for i := range calls {
		if s.LivekitEnabled && calls[i].RoomName != "" {
			go s.all.Livekit.DeleteRoom(calls[i].RoomName)
		}

		msg := data.Message{
			ChatID: calls[i].ChatID,
			UserID: calls[i].InitiatorID,
		}

		s.setEndCallInfo(&calls[i], &msg)
		s.all.Informer.SendSignalToCall(&calls[i], status)
		s.all.Informer.SendMessageEvent(calls[i].ChatID, &msg, "", 0, true)
	}

	return nil
}

func (s *baseCallService) checkForActiveCall(ctx *CallContext, targetChatId int, targetUserId int) error {
	// check if the current user is already in call
	call, err := s.dao.Calls.GetByUser(ctx.UserID)
	if err != nil {
		return err
	}

	if call.ID != 0 {
		cu := call.GetByUserID(ctx.UserID)
		if cu == nil {
			return errors.New("user not found in the call")
		}

		if call.ChatID != targetChatId {
			return errActiveInOtherChat // call is active in another chat
		}

		if cu.DeviceID == ctx.DeviceID {
			return errAlreadyInCall // alreay in the call
		}
	}

	return nil
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
		s.setEndCallInfo(call, msg)
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

	userStatus := data.CallUserStatusConnecting
	if cu.Status == data.CallUserStatusConnecting ||
		call.Status == data.CallStatusInitiated && cu.Status == data.CallUserStatusInitiated {
		userStatus = data.CallUserStatusActive
	}

	err := s.dao.CallUsers.UpdateUserDeviceID(call.ID, ctx.UserID, ctx.DeviceID, userStatus)

	// join for the first time of from another device
	notify := cu.DeviceID == 0 || cu.DeviceID != ctx.DeviceID

	cu.Status = userStatus
	cu.DeviceID = ctx.DeviceID

	return notify, err
}

func (s *baseCallService) setEndCallInfo(call *data.Call, msg *data.Message) {
	var diff float64
	if call.Start != nil {
		diff = time.Since(*call.Start).Seconds()
		msg.Date = *call.Start
	}
	msg.Text = fmt.Sprintf("%02d:%02d", int(math.Floor(diff/60)), int(diff)%60)
	msg.Type = data.CallStartMessage
}

func (s *baseCallService) end(c *data.Call) error {
	if s.LivekitEnabled {
		// should delete the room as the call has been ended
		go s.all.Livekit.DeleteRoom(c.RoomName)
	}
	return s.dao.CallUsers.EndCall(c.ID)
}
