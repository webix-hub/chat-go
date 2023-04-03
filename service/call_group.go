package service

import (
	"fmt"
	"mkozhukh/chat/data"
	"time"
)

type Call struct {
	ID          int        `json:"id"`
	Status      int        `json:"status"`
	InitiatorID int        `json:"initiator"`
	ChatID      int        `json:"chat"`
	Start       *time.Time `json:"start"`
	IsGroupCall bool       `json:"group"`
	Name        string     `json:"name"`
	Avatar      string     `json:"avatar"`
	Users       []int      `json:"users"`
}

type groupCallService struct {
	baseCallService
}

func newGroupCallsService(base baseCallService) *groupCallService {
	return &groupCallService{base}
}

func (s *groupCallService) Start(ctx *CallContext, targetChatId, targetUserId int) (*data.Call, error) {
	if !s.LivekitEnabled {
		return nil, data.ErrFeatureDisabled
	}

	c, err := s.precheckUserAccess(ctx, targetChatId, targetUserId)
	if err != nil {
		return c, err
	}

	// check if the chat is in active call
	call, err := s.dao.Calls.CheckIfChatInCall(targetChatId)
	if err != nil {
		return nil, err
	}
	if call.ID != 0 {
		// join the user to the existing call
		err := s.Join(ctx, &call)
		return &call, err
	}

	// start new call
	call, err = s.dao.Calls.Start(ctx.UserID, ctx.DeviceID, 0, targetChatId)
	if err != nil {
		return nil, err
	}

	err = s.createRoom(&call)
	if err != nil {
		return nil, err
	}
	s.all.Informer.SendSignalToCall(&call, call.Status)
	s.StartCallTimer(s.notAcceptedTimeout, call.ID, s.dropNotAcceptedHandler)

	return &call, err
}

func (s *groupCallService) Join(ctx *CallContext, call *data.Call) error {
	notify, err := s.updateAcceptedCall(ctx, call)
	if err != nil {
		return err
	}

	toUsers := []data.CallUser{{UserID: ctx.UserID, DeviceID: ctx.DeviceID}}
	if call.Status == data.CallStatusInitiated && notify {
		// when the first user other than the initiator joins the call,
		// should notify the initiator that the call has been accepted
		cu := call.GetByUserID(call.InitiatorID)
		if cu != nil {
			toUsers = append(toUsers, data.CallUser{UserID: cu.UserID, DeviceID: cu.DeviceID})
		}
		// change call status to 'active'
		err = s.updateStatusAndSendMessage(call, data.CallStatusAccepted)
		if err != nil {
			return err
		}
	}

	s.all.Informer.SendSignalToCall(call, call.Status, toUsers...)

	if notify {
		// inform other devices to end the incoming call
		// as it is already accepted on the current device
		s.all.Informer.SendSignalToUser(ctx.UserID, CallDevices{
			Devices: call.GetDevicesIDs(false),
		})
	}

	return nil
}

func (s *groupCallService) Disconnect(ctx *CallContext, call *data.Call, status int) error {
	if !s.LivekitEnabled {
		return data.ErrFeatureDisabled
	}
	if call.Status > 900 {
		return fmt.Errorf("call already ended")
	}

	if call.Status == data.CallStatusInitiated && call.InitiatorID == ctx.UserID {
		// reject call by the initiator
		err := s.updateStatusAndSendMessage(call, data.CallStatusRejected)
		s.all.Informer.SendSignalToCall(call, data.CallStatusRejected)
		return err
	}

	activeCount := 0
	connectingCount := 0
	for i := range call.Users {
		cu := &call.Users[i]

		if cu.UserID == ctx.UserID {
			// update connection state
			cu.Status = data.CallUserStatusDisconnected
		}

		if cu.Status == data.CallUserStatusActive {
			activeCount++
		}
		if cu.Status == data.CallUserStatusConnecting {
			connectingCount++
		}
	}

	err := s.dao.CallUsers.UpdateUserDeviceID(call.ID, ctx.UserID, ctx.DeviceID, data.CallUserStatusDisconnected)
	if err != nil {
		return err
	}

	toUsers := []data.CallUser{{UserID: ctx.UserID, DeviceID: ctx.DeviceID}}
	if call.Status == data.CallStatusActive && activeCount == 0 || (connectingCount == 0 && activeCount == 0) {
		// if the last participant has been disconnected, then end the call
		toUsers = call.Users
		err := s.updateStatusAndSendMessage(call, data.CallStatusEnded)
		if err != nil {
			return err
		}
	}

	// notify the current user to end the call
	s.all.Informer.SendSignalToCall(call, data.CallStatusDisconnected, toUsers...)

	return err
}

func (s *groupCallService) RefreshCallUsers(chatId int, users []int) error {
	if !s.LivekitEnabled {
		return nil
	}

	call, err := s.dao.Calls.CheckIfChatInCall(chatId)
	if err != nil {
		return err
	}

	_, deleted, err := s.dao.Calls.RefreshCallUsers(&call, users)
	if err != nil {
		return err
	}

	if len(deleted) > 0 {
		// notify call participants to disconnect users that where deleted from the chat
		s.all.Informer.SendSignalToCall(&call, data.CallStatusDisconnected, deleted...)
	}

	return nil
}

func (s *groupCallService) dropNotAcceptedHandler(id int) {
	call, err := s.dao.Calls.Get(id)
	if err != nil {
		return
	}

	notAcceptedUsers := make([]data.CallUser, 0)
	for _, u := range call.Users {
		if u.Status == data.CallUserStatusInitiated && u.DeviceID == 0 {
			notAcceptedUsers = append(notAcceptedUsers, u)
		}
	}
	// notify users who not accepted the call to drop it
	s.all.Informer.SendSignalToCall(&call, data.CallStatusIgnored, notAcceptedUsers...)
}
