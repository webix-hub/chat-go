package api

import (
	"mkozhukh/chat/data"
	"mkozhukh/chat/service"
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

type CallsAPI struct {
	db   *data.DAO
	sAll *service.ServiceAll
}

func (d *CallsAPI) Start(targetUserId int, targetChatId int, ctx *service.CallContext) (*Call, error) {
	callService, err := service.CallProvider.GetService(targetUserId == 0)
	if err != nil {
		return nil, err
	}

	call, err := callService.Start(ctx, targetChatId, targetUserId)
	if err != nil {
		return nil, err
	}

	return &Call{
		ID:          call.ID,
		Status:      call.Status,
		Start:       call.Start,
		InitiatorID: call.InitiatorID,
		IsGroupCall: call.IsGroupCall,
		Users:       call.GetUsersIDs(false),
	}, nil
}

func (d *CallsAPI) SetStatus(id, status int, ctx *service.CallContext) (int, error) {
	call, err := d.db.Calls.Get(id)
	if err != nil {
		return 0, err
	}

	callService, err := service.CallProvider.GetService(call.IsGroupCall)
	if err != nil {
		return 0, err
	}
	
	if status == data.CallStatusAccepted {
		err = callService.Join(ctx, &call)
	}

	if status > 900 {
		err = callService.Disconnect(ctx, &call, status)
	}

	return call.Status, err
}

func (d *CallsAPI) SetUserStatus(id, status int, ctx *service.CallContext) error {
	call, err := d.db.Calls.Get(id)
	if err != nil {
		return err
	}
	if call.Status > 900 {
		return nil
	}
	user := call.GetByUserID(ctx.UserID)
	if user == nil {
		return err
	}
	if user.Status == status {
		return nil
	}

	err = d.db.CallUsers.UpdateUserConnState(id, ctx.UserID, status)

	if status == data.CallUserStatusConnecting {
		d.sAll.Calls.StartReconnectingTimer(ctx, id)
	}

	return err
}

func (d *CallsAPI) Signal(signalType, msg string, ctx *service.CallContext) error {
	call, err := d.db.Calls.GetByDevice(ctx.DeviceID)
	if err != nil {
		return err
	}
	if call.ID == 0 {
		return data.ErrAccessDenied
	}

	i := 0
	if call.Users[0].DeviceID == ctx.DeviceID {
		i = 1
	}
	to := call.Users[i].UserID
	toDevice := call.Users[i].DeviceID

	d.sAll.Informer.SendSignal(signalType, msg, []int{to}, []int{toDevice})

	return nil
}

func (d *CallsAPI) JoinToken(callId int, ctx *service.CallContext) (string, error) {
	return d.sAll.Calls.CreateJoinToken(ctx, callId)
}
