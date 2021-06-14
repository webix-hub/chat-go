package api

import (
	"fmt"
	remote "github.com/mkozhukh/go-remote"
	"mkozhukh/chat/data"
	"time"
)

type Call struct {
	ID      int        `json:"id"`
	Status  int        `json:"status"`
	Users   []int      `json:"users"`
	Devices []int      `json:"devices"`
	Start   *time.Time `json:"start"`
}

type CallsAPI struct {
	db      *data.DAO
	service *CallService
}

type Signal struct {
	Message string `json:"msg"`
	Type    string `json:"type"`
	Users   []int  `json:"-"`
	Devices []int  `json:"-"`
}

func (d *CallsAPI) Start(targetUserId int, chatId int, userId UserID, device DeviceID) (*Call, error) {
	call, err := d.db.Calls.Start(int(userId), int(device), targetUserId, chatId)
	if err != nil {
		return nil, err
	}

	d.service.sendEvent(&call)
	d.service.StartCall(call.ID)

	return &Call{
		ID:      call.ID,
		Status:  call.Status,
		Users:   []int{call.FromUserID, call.ToUserID},
		Devices: []int{call.FromDeviceID, call.ToDeviceID},
		Start:   nil,
	}, nil
}

func (d *CallsAPI) SetStatus(id, status int, userId UserID, deviceID DeviceID) (int, error) {
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

	if status == data.CallStatusAccepted && call.ToDeviceID == 0 {
		call.ToDeviceID = int(deviceID)
	}

	err = d.service.callStatusUpdate(&call, status)
	if err != nil {
		return 0, err
	}

	d.service.sendEvent(&call)

	return call.Status, nil
}

func (d *CallsAPI) Signal(signalType, msg string, device DeviceID, events *remote.Hub) error {
	call, err := d.db.Calls.GetByDevice(int(device))
	if err != nil {
		return fmt.Errorf("%s", "Access denied")
	}

	var to, toDevice int
	if call.FromDeviceID == int(device) {
		to = call.ToUserID
		toDevice = call.ToDeviceID
	} else {
		to = call.FromUserID
		toDevice = call.FromDeviceID
	}

	events.Publish("signal", Signal{
		Type:    signalType,
		Message: msg,
		Users:   []int{to},
		Devices: []int{toDevice},
	})

	return nil
}
