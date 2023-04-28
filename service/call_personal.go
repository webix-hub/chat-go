package service

import (
	"mkozhukh/chat/data"
)

type personalCallService struct {
	baseCallService
}

func newPersonalCallService(base baseCallService) *personalCallService {
	return &personalCallService{base}
}

func (s *personalCallService) Start(ctx *CallContext, targetChatId, targetUserId int) (*data.Call, error) {
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

	c, err := s.checkUserBusy(ctx, targetChatId, targetUserId)
	if err != nil {
		return c, err
	}

	call, err = s.dao.Calls.Start(ctx.UserID, ctx.DeviceID, targetUserId, targetChatId)
	if err != nil {
		return nil, err
	}

	if s.LivekitEnabled {
		err = s.createRoom(&call)
		if err != nil {
			return nil, err
		}
	}
	s.all.Informer.SendSignalToCall(&call, call.Status)
	s.StartCallTimer(s.notAcceptedTimeout, call.ID, s.dropNotAcceptedHandler)

	return &call, err
}

func (s *personalCallService) Join(ctx *CallContext, call *data.Call) error {
	notify, err := s.updateAcceptedCall(ctx, call)
	if err != nil {
		return err
	}

	if call.Status == data.CallStatusInitiated {
		err = s.updateStatusAndSendMessage(call, data.CallStatusAccepted)
		if err != nil {
			return err
		}
	}

	s.all.Informer.SendSignalToCall(call, call.Status)

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

func (s *personalCallService) Disconnect(ctx *CallContext, call *data.Call, status int) error {
	err := s.updateStatusAndSendMessage(call, status)
	if err == nil {
		s.all.Informer.SendSignalToCall(call, status)
	}

	return err
}

func (s *personalCallService) dropNotAcceptedHandler(id int) {
	call, err := s.dao.Calls.Get(id)
	if err != nil {
		return
	}
	if call.Status == data.CallStatusInitiated {
		s.updateStatusAndSendMessage(&call, data.CallStatusIgnored)
		// notify all users to drop incoming call
		s.all.Informer.SendSignalToCall(&call, data.CallStatusIgnored)
	}
}

func (s *personalCallService) checkUserBusy(ctx *CallContext, toChatId, toUserId int) (*data.Call, error) {
	call, err := s.dao.Calls.GetByUser(toUserId)
	if err != nil {
		return nil, err
	}
	if call.ID != 0 {
		call := data.Call{
			InitiatorID: ctx.UserID,
			Status:      data.CallStatusBusy,
			ChatID:      toChatId,
		}
		s.SendCallMessage(&call, data.CallBusyMessage)
		return &call, errLineIsBusy
	}

	return nil, nil
}
