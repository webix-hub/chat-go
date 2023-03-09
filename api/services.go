package api

import (
	"encoding/json"
	"fmt"
	"math"
	"mkozhukh/chat/data"
	"time"

	gonanoid "github.com/matoous/go-nanoid"
	remote "github.com/mkozhukh/go-remote"
)

type CallService struct {
	cDAO   data.CallsDAO
	cuDAO  data.CallUsersDAO
	mDAO   data.MessagesDAO
	chDAO  data.ChatsDAO
	uchDAO data.UserChatsDAO
	hub    *remote.Hub

	livekit *LivekitService

	offlineDevices map[int]time.Time
}

func newCallService(
	cdao data.CallsDAO,
	cudao data.CallUsersDAO,
	mdao data.MessagesDAO,
	chdao data.ChatsDAO,
	uchdao data.UserChatsDAO,
	hub *remote.Hub,
	livekit *LivekitService,
) *CallService {
	d := CallService{
		cDAO:           cdao,
		cuDAO:          cudao,
		mDAO:           mdao,
		chDAO:          chdao,
		uchDAO:         uchdao,
		hub:            hub,
		livekit:        livekit,
		offlineDevices: make(map[int]time.Time),
	}
	go d.runCheckOfflineUsers()

	return &d
}

func (d *CallService) StartCall(id int) {
	time.AfterFunc(30*time.Second, func() { d.dropNotAccepted(id) })
}

func (d *CallService) dropNotAccepted(id int) {
	call, err := d.cDAO.Get(id)
	if err != nil {
		return
	}

	if call.Status == data.CallStatusInitiated {
		_ = d.callStatusUpdate(&call, data.CallStatusIgnored)
		d.sendEvent(&call)
	}
}

func (d *CallService) ChangeOnlineStatus(device int, status int) {
	if status == data.StatusOnline {
		delete(d.offlineDevices, device)
		return
	}

	if status == data.StatusOffline {
		d.offlineDevices[device] = time.Now()
	}
}

func (d *CallService) runCheckOfflineUsers() {
	for range time.Tick(time.Second * 10) {
		d.checkOfflineUsers()
	}
}

func (d *CallService) checkOfflineUsers() {
	if len(d.offlineDevices) == 0 {
		return
	}

	check := time.Now().Add(-15 * time.Second)
	for key, offTime := range d.offlineDevices {
		if offTime.Before(check) {
			c, err := d.cDAO.GetByDevice(key)
			if err != nil {
				return
			}

			_ = d.callStatusUpdate(&c, data.CallStatusLost)
			d.sendEvent(&c)

			delete(d.offlineDevices, key)
		}
	}
}

func (d *CallService) sendEvent(c *data.Call, to ...data.CallUser) {
	msg, _ := json.Marshal(&Call{
		ID:          c.ID,
		Status:      c.Status,
		Start:       c.Start,
		InitiatorID: c.InitiatorID,
		IsGroupCall: c.IsGroupCall,
		ChatID:      c.ChatID,
		Users:       c.GetUsersIDs(),
	})

	var devices []int
	var users []int
	if to != nil {
		for _, cu := range to {
			devices = append(devices, cu.DeviceID)
			users = append(users, cu.UserID)
		}
	} else {
		devices = c.GetDevicesIDs()
		users = c.GetUsersIDs()
	}

	d.hub.Publish("signal", Signal{
		Type:    "connect",
		Message: string(msg),
		Users:   users,
		Devices: devices,
	})
}

func (d *CallService) broadcastToUserDevices(targetUser int, payload interface{}) {
	msg, _ := json.Marshal(&payload)
	d.hub.Publish("signal", Signal{
		Type:    "connect",
		Message: string(msg),
		Users:   []int{targetUser},
		Devices: []int{0},
	})
}

func (d *CallService) callStatusUpdate(c *data.Call, status int) (err error) {
	defer func() {
		if err != nil || status > 900 {
			e := d.endCall(c)
			if e != nil {
				fmt.Printf("LIVEKIT ERROR: %s\n", e.Error())
			}
		}
	}()

	err = d.cDAO.Update(c, status)
	if err != nil {
		return err
	}

	if (status == data.CallStatusEnded || status == data.CallStatusLost) && c.ChatID != 0 {
		diff := time.Since(*c.Start).Seconds()
		msg := &data.Message{
			Date:   *c.Start,
			Text:   fmt.Sprintf("%02d:%02d", int(math.Floor(diff/60)), int(diff)%60),
			ChatID: c.ChatID,
			UserID: c.InitiatorID,
			Type:   data.CallStartMessage,
		}
		err = d.mDAO.SaveAndSend(c.ChatID, msg, "", 0)
		if err != nil {
			return err
		}
	}

	if (status == data.CallStatusRejected) && c.ChatID != 0 {
		return d.RejectCall(c)
	}

	if (status == data.CallStatusIgnored) && c.ChatID != 0 {
		msg := &data.Message{
			Text:   "",
			ChatID: c.ChatID,
			UserID: c.InitiatorID,
			Type:   data.CallMissedMessage,
		}

		err = d.mDAO.SaveAndSend(c.ChatID, msg, "", -1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *CallService) sendMessage(c *data.Call, msg *data.Message, err error) error {
	d.hub.Publish("messages", data.MessageEvent{Op: "add", Msg: msg})

	err = d.uchDAO.IncrementCounter(c.ChatID, int(c.InitiatorID))
	if err != nil {
		return err
	}

	_, err = d.chDAO.SetLastMessage(c.ChatID, msg)
	if err != nil {
		return err
	}

	return nil
}

func (d *CallService) createRoom(c *data.Call) error {
	c.RoomName, _ = gonanoid.ID(16)
	err := d.cDAO.Save(c)
	if err != nil {
		return err
	}

	_, err = d.livekit.CreateRoom(c.RoomName)
	if err != nil {
		c.Status = data.CallStatusLost
		d.cDAO.Save(c)
	}

	return err
}

func (d *CallService) RejectCall(c *data.Call) error {
	msg := &data.Message{
		Text:   "",
		ChatID: c.ChatID,
		UserID: c.InitiatorID,
		Type:   data.CallRejectedMessage,
	}
	err := d.mDAO.Save(msg)
	if err != nil {
		return err
	}

	err = d.sendMessage(c, msg, err)
	if err != nil {
		return err
	}

	return nil
}

func (d *CallService) endCall(c *data.Call) error {
	if LIVEKIT_ENABLED {
		// should delete room as the call has been ended
		go d.livekit.DeleteRoom(c.RoomName)
	}

	err := d.cuDAO.EndCall(c.ID)
	return err
}
