package service

import "mkozhukh/chat/data"

type ICallService interface {
	Start(ctx *CallContext, targetChatId, targetUserId int) (*data.Call, error)
	Join(ctx *CallContext, c *data.Call) error
	Disconnect(ctx *CallContext, c *data.Call, status int) error
}

type CallContext struct {
	UserID   int
	DeviceID int
}

type CallServiceProvider struct {
	group    *groupCallService
	personal *personalCallService
}

var CallProvider *CallServiceProvider

func (p *CallServiceProvider) GetService(group bool) ICallService {
	if group {
		return p.group
	}
	return p.personal
}
