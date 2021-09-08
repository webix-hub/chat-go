package api

import (
	"encoding/json"
	"fmt"
	remote "github.com/mkozhukh/go-remote"
	"math"
	"mkozhukh/chat/data"
	"time"
)

type CallService struct {
	cDAO   data.CallsDAO
	mDAO   data.MessagesDAO
	chDAO  data.ChatsDAO
	uchDAO data.UserChatsDAO
	hub    *remote.Hub

	offlineDevices map[int]time.Time
}

func newCallService(cdao data.CallsDAO, mdao data.MessagesDAO, chdao data.ChatsDAO, uchdao data.UserChatsDAO, hub *remote.Hub) *CallService {
	d := CallService{
		cDAO:           cdao,
		mDAO:           mdao,
		chDAO:          chdao,
		uchDAO:         uchdao,
		hub:            hub,
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
func (d *CallService) sendEvent(c *data.Call) {
	msg, _ := json.Marshal(&Call{
		ID:      c.ID,
		Status:  c.Status,
		Start:   c.Start,
		Users:   []int{c.FromUserID, c.ToUserID},
		Devices: []int{c.FromDeviceID, c.ToDeviceID},
	})
	d.hub.Publish("signal", Signal{
		Type:    "connect",
		Message: string(msg),
		Users:   []int{c.FromUserID, c.ToUserID},
		Devices: []int{c.FromDeviceID, c.ToDeviceID},
	})
}

func (d *CallService) callStatusUpdate(c *data.Call, status int) error {
	err := d.cDAO.Update(c, status)
	if err != nil {
		return err
	}

	if (status == data.CallStatusEnded || status == data.CallStatusLost) && c.ChatID != 0 {
		diff := time.Now().Sub(*c.Start).Seconds()
		msg := &data.Message{
			Date:   *c.Start,
			Text:   fmt.Sprintf("%02d:%02d", int(math.Floor(diff/60)), int(diff)%60),
			ChatID: c.ChatID,
			UserID: c.FromUserID,
			Type:   data.CallStartMessage,
		}
		err = d.mDAO.SaveAndSend(c.ChatID, msg)
		if err != nil {
			return err
		}
	}

	if (status == data.CallStatusRejected) && c.ChatID != 0 {
		msg := &data.Message{
			Text:   "",
			ChatID: c.ChatID,
			UserID: c.FromUserID,
			Type:   data.CallRejectedMessage,
		}

		err = d.mDAO.SaveAndSend(c.ChatID, msg)
		if err != nil {
			return err
		}
	}

	if (status == data.CallStatusIgnored) && c.ChatID != 0 {
		msg := &data.Message{
			Text:   "",
			ChatID: c.ChatID,
			UserID: c.FromUserID,
			Type:   data.CallMissedMessage,
		}

		err = d.mDAO.SaveAndSend(c.ChatID, msg)
		if err != nil {
			return err
		}
	}

	return nil
}
