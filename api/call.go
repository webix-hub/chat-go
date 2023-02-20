package api

import (
	"fmt"
	"mkozhukh/chat/data"
	"strconv"
	"time"

	remote "github.com/mkozhukh/go-remote"
)

var LIVEKIT_ENABLED = false

var nRoom = time.Now().Unix()

type Call struct {
	ID          int        `json:"id"`
	Status      int        `json:"status"`
	InitiatorID int        `json:"initiator"`
	Users       []int      `json:"users"`
	IsGroupCall bool       `json:"group"`
	Start       *time.Time `json:"start"`
}

type CallDevices struct {
	Devices []int `json:"devices"`
}

type CallsAPI struct {
	db      *data.DAO
	service *CallService
	livekit *LivekitService
}

type Signal struct {
	Message string `json:"msg"`
	Type    string `json:"type"`
	Users   []int  `json:"-"`
	Devices []int  `json:"-"`
}

func (d *CallsAPI) Start(targetUserId int, chatId int, userId UserID, device DeviceID) (*Call, error) {
	isGroupChat := targetUserId == 0
	if isGroupChat && !LIVEKIT_ENABLED {
		return nil, data.ErrFeatureDisabled
	}

	call, err := d.db.Calls.Start(int(userId), int(device), targetUserId, chatId)
	if err != nil {
		return nil, err
	}

	if call.Status != data.CallStatusRejected {
		if LIVEKIT_ENABLED {
			err = d.service.createRoom(&call)
			if err != nil {
				return nil, err
			}
		}
		d.service.sendEvent(&call)
		d.service.StartCall(&call)
	} else {
		d.service.RejectCall(&call)
	}

	return &Call{
		ID:          call.ID,
		Status:      call.Status,
		Start:       nil,
		InitiatorID: call.InitiatorID,
		IsGroupCall: isGroupChat,
		Users:       call.GetUsersIDs(),
	}, nil
}

func (d *CallsAPI) SetStatus(id, status int, userId UserID, deviceId DeviceID, hub *remote.Hub) (int, error) {
	call, err := d.db.Calls.Get(id)
	if err != nil {
		return 0, err
	}

	// check if user has access to this call
	err = d.checkCallAccess(call.ChatID, int(userId))
	if err != nil {
		return 0, err
	}

	needToInformOthers := status == data.CallStatusAccepted
	if needToInformOthers {
		needToInformOthers, err = d.updateAcceptedCall(&call, int(userId), int(deviceId))
		if err != nil {
			return 0, err
		}
	}

	var toUsers []data.CallUser

	if call.IsGroupCall {
		toUsers = []data.CallUser{
			{UserID: int(userId), DeviceID: int(deviceId)},
		}

		if call.Status == data.CallStatusInitiated {
			// when the first user (excepts initiator) connects to the call,
			// should notify initiator that call has been started
			for _, cu := range call.Users {
				if cu.UserID == call.InitiatorID {
					toUsers = append(toUsers, data.CallUser{
						UserID:   cu.UserID,
						DeviceID: cu.DeviceID,
					})
					break
				}
			}
		}
	}

	err = d.service.callStatusUpdate(&call, status)
	if err != nil {
		return 0, err
	}

	d.service.sendEvent(&call, toUsers...)

	if needToInformOthers {
		callDevices := make([]int, len(call.Users))
		for i := range callDevices {
			callDevices[i] = call.Users[i].DeviceID
		}
		// inform other devices to end incoming call as it is already accepted on the current device
		d.service.broadcastToUserDevices(int(userId), CallDevices{
			Devices: callDevices,
		})
	}

	return call.Status, nil
}

func (d *CallsAPI) Signal(signalType, msg string, device DeviceID, events *remote.Hub) error {
	call, err := d.db.Calls.GetByDevice(int(device))
	if err != nil {
		return fmt.Errorf("%s", "Access denied")
	}

	usersTo := make([]int, len(call.Users)-1)
	devicesTo := make([]int, len(call.Users)-1)
	for i, u := range call.Users {
		if u.DeviceID != int(device) {
			usersTo[i] = u.UserID
			devicesTo[i] = u.DeviceID
		}
	}

	events.Publish("signal", Signal{
		Type:    signalType,
		Message: msg,
		Users:   usersTo,
		Devices: devicesTo,
	})

	return nil
}

func (d *CallsAPI) Disconnect(callId int, userId UserID, device DeviceID) error {
	call, err := d.db.Calls.Get(callId)
	if err != nil {
		return err
	}
	if call.Status > 900 {
		return fmt.Errorf("call already ended")
	}

	if call.InitiatorID == int(userId) && call.Status == data.CallStatusInitiated {
		// the call has not started yet
		err := d.service.callStatusUpdate(&call, data.CallStatusRejected)
		d.service.sendEvent(&call)
		return err
	}

	isLastParticipant := true
	for i := range call.Users {
		cu := &call.Users[i]

		if cu.UserID == int(userId) {
			// update connection state
			cu.Connected = false
			if cu.DeviceID == 0 {
				// update deviceId if it was not defined
				err = d.db.CallUsers.UpdateUserDeviceID(call.ID, int(userId), int(device))
				if err != nil {
					return err
				}
			}
		}

		if isLastParticipant && cu.Connected {
			isLastParticipant = false
		}
	}

	err = d.db.CallUsers.UpdateUserConnState(call.ID, int(userId), false)
	if err != nil {
		return err
	}

	if isLastParticipant {
		// as the last participant has been disconnected, then end the call
		return d.service.callStatusUpdate(&call, data.CallStatusEnded)
	}

	call.Status = data.CallStatusDisconnected
	// notify the the current client to end the call
	d.service.sendEvent(&call, data.CallUser{UserID: int(userId), DeviceID: int(device)})

	return nil
}

func (d *CallsAPI) JoinToken(callId int, userId UserID, device DeviceID) (string, error) {
	if !LIVEKIT_ENABLED {
		return "", data.ErrFeatureDisabled
	}

	call, err := d.service.cDAO.Get(callId)
	if err != nil {
		return "", err
	}

	err = d.checkCallAccess(call.ChatID, int(userId))
	if err != nil {
		return "", err
	}

	token, err := d.livekit.GetJoinToken(call.RoomName, strconv.Itoa(int(userId)))

	return token, err
}

func (d *CallsAPI) checkCallAccess(chatId, userId int) error {
	chatusers, err := d.db.UserChats.ByChat(chatId)
	if err != nil {
		return err
	}

	canModify := false
	for _, u := range chatusers {
		if u.UserID == int(userId) {
			canModify = true
			break
		}
	}

	if !canModify {
		return fmt.Errorf("%s", "Access denied")
	}

	return nil
}

func (d *CallsAPI) updateAcceptedCall(call *data.Call, userId, deviceId int) (bool, error) {
	for i, cu := range call.Users {
		if cu.UserID == userId {
			if cu.DeviceID != 0 {
				// if user is already in call, update connection state
				err := d.db.CallUsers.UpdateUserConnState(call.ID, userId, true)
				return false, err
			} else {
				// user accepts the call for the first time, then should add info about him
				call.Users[i].DeviceID = deviceId
				err := d.db.CallUsers.UpdateUserDeviceID(call.ID, userId, deviceId)
				return true, err
			}
		}
	}

	return false, fmt.Errorf("access denied")
}

func newRoomName() string {
	// TODO use UID (nonouid)
	nRoom++
	return fmt.Sprintf("room-%d", nRoom)
}
