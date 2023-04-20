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

	err := s.checkUserAccess(targetChatId, ctx.UserID)
	if err != nil {
		return nil, err
	}

	err = s.checkForActiveCall(ctx, targetChatId, targetUserId)
	if err != nil {
		return nil, err
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
	if call.Status == data.CallStatusInitiated {
		cu := call.GetByUserID(ctx.UserID)
		if cu.Status == data.CallUserStatusActive {
			// when the first user other than the initiator joins the call,
			// should notify the initiator that the call has been accepted
			initiator := call.GetByUserID(call.InitiatorID)
			if initiator != nil {
				toUsers = append(toUsers, data.CallUser{UserID: initiator.UserID, DeviceID: initiator.DeviceID})
			}
			// change call status to 'active'
			err = s.updateStatusAndSendMessage(call, data.CallStatusAccepted)
			if err != nil {
				return err
			}
		}
	}

	s.all.Informer.SendSignalToCall(call, call.Status, toUsers...)

	if notify {
		// inform other devices to end the incoming call
		// as it is already accepted on the current device
		s.all.Informer.SendSignalToUser(ctx.UserID, CallDevices{
			Message: "Joined from another deivce",
			Devices: call.GetDevicesIDs(false),
		})
	} else {
		s.all.Calls.StartReconnectingTimer(ctx, call.ID)
	}

	return nil
}

func (s *groupCallService) Disconnect(ctx *CallContext, call *data.Call, status int) error {
	if !s.LivekitEnabled {
		return data.ErrFeatureDisabled
	}
	if call.Status > 900 {
		s.all.Informer.SendSignalToCall(call, data.CallStatusDisconnected, data.CallUser{UserID: ctx.UserID})
		return fmt.Errorf("call already ended")
	}

	if call.Status == data.CallStatusInitiated && call.InitiatorID == ctx.UserID {
		// reject call by the initiator
		err := s.updateStatusAndSendMessage(call, data.CallStatusRejected)
		s.all.Informer.SendSignalToCall(call, data.CallStatusRejected)
		return err
	}

	var currentStatus int

	activeCount := 0
	connectingCount := 0
	for i := range call.Users {
		cu := &call.Users[i]

		if cu.UserID == ctx.UserID {
			// update user status
			currentStatus = cu.Status
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

	drop := false
	if currentStatus == data.CallUserStatusActive && activeCount == 0 {
		// last active user has been disconnected,
		// then end the call for all users (including connecting/initiated statuses)
		drop = true
	} else if activeCount == 0 && connectingCount == 0 {
		drop = true
	}

	toUsers := []data.CallUser{}
	if drop {
		// notify all not disconnected users
		toUsers, err = s.dao.CallUsers.GetNotDisconnectedCallUsers(call.ID)
		if err != nil {
			return err
		}

		// if the last participant has been disconnected, then end the call
		err := s.updateStatusAndSendMessage(call, data.CallStatusEnded)
		if err != nil {
			return err
		}
	}
	toUsers = append(toUsers, data.CallUser{UserID: ctx.UserID, DeviceID: ctx.DeviceID})

	// notify the current user to end the call
	s.all.Informer.SendSignalToCall(call, data.CallStatusDisconnected, toUsers...)

	// remove participant from the room
	go s.all.Livekit.DisconnectParticipant(call.RoomName, fmt.Sprint(ctx.UserID))

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
	if call.ID == 0 || call.Status > 900 {
		return nil
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

	if call.ID == 0 || call.Status > 900 {
		return
	}

	notAcceptedUsers := make([]data.CallUser, 0)
	for _, u := range call.Users {
		if u.Status == data.CallUserStatusInitiated && u.DeviceID == 0 {
			s.dao.CallUsers.UpdateUserConnState(call.ID, u.UserID, data.CallUserStatusDisconnected)
			notAcceptedUsers = append(notAcceptedUsers, u)
		}
	}
	// notify users who not accepted the call to drop it
	s.all.Informer.SendSignalToCall(&call, data.CallStatusIgnored, notAcceptedUsers...)
}
