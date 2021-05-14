package api

import (
	"fmt"
	remote "github.com/mkozhukh/go-remote"
	"mkozhukh/chat/data"
	"time"
)

type Call struct {
	ID     int        `json:"id"`
	Status int        `json:"status"`
	Users  []int      `json:"users"`
	Start  *time.Time `json:"start"`
}

type CallsAPI struct {
	db      *data.DAO
	service *CallService
}

type Signal struct {
	Message string `json:"msg"`
	Type    string `json:"type"`
	Users   []int  `json:"-"`
}

func (d *CallsAPI) Start(targetUserId int, userId UserID) (*Call, error) {
	call, err := d.db.Calls.Start(int(userId), targetUserId)
	if err != nil {
		return nil, err
	}

	d.service.sendEvent(&call)
	d.service.StartCall(call.ID)

	return &Call{
		ID:     call.ID,
		Status: call.Status,
		Users:  []int{call.FromUserID, call.ToUserID},
		Start:  nil,
	}, nil
}

func (d *CallsAPI) SetStatus(id, status int, userId UserID) (int, error) {
	call, err := d.db.Calls.Get(id)
	if err != nil {
		return 0, err
	}

	canModify := false
	if call.FromUserID == int(userId) || call.ToUserID == int(userId) {
		canModify = true
	}

	if !canModify {
		return 0, fmt.Errorf("%s", "Access denied")
	}

	err = d.service.callStatusUpdate(&call, status)
	if err != nil {
		return 0, err
	}

	d.service.sendEvent(&call)

	return call.Status, nil
}

func (d *CallsAPI) Signal(signalType, msg string, userId UserID, events *remote.Hub) error {
	call, err := d.db.Calls.GetByUser(int(userId))
	if err != nil {
		return fmt.Errorf("%s", "Access denied")
	}

	to := call.ToUserID
	if to == int(userId) {
		to = call.FromUserID
	}

	events.Publish("signal", Signal{
		Type:    signalType,
		Message: msg,
		Users:   []int{to},
	})

	return nil
}
