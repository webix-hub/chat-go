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
	c, err := s.precheckUserAccess(ctx, targetChatId, targetUserId)
	if err != nil {
		return c, err
	}

	call, err := s.dao.Calls.Start(ctx.UserID, ctx.DeviceID, targetUserId, targetChatId)
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
			Devices: call.GetDevicesIDs(false),
		})
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
