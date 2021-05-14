package api

import (
	"encoding/json"
	remote "github.com/mkozhukh/go-remote"
	"mkozhukh/chat/data"
	"time"
)

type CallService struct {
	cDAO data.CallsDAO
	mDAO data.MessagesDAO
	hub *remote.Hub

	offlineUsers map[int]time.Time
}

func newCallService(cdao data.CallsDAO, mdao data.MessagesDAO, hub *remote.Hub) *CallService {
	d := CallService{cDAO: cdao, mDAO:mdao, hub: hub, offlineUsers: make(map[int]time.Time)}
	go d.runCheckOfflineUsers()

	return &d
}

func (d *CallService) StartCall(id int) {
	time.AfterFunc(10*time.Second, func() { d.dropNotAccepted(id) })
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

func (d *CallService) ChangeOnlineStatus(user int, status int, events *remote.Hub) {
	if status == data.StatusOnline {
		delete(d.offlineUsers, user)
		return
	}

	if status == data.StatusOffline {
		d.offlineUsers[user] = time.Now()
	}
}

func (d *CallService) runCheckOfflineUsers() {
	for range time.Tick(time.Second * 10) {
		d.checkOfflineUsers()
	}
}

func (d *CallService) checkOfflineUsers() {
	if len(d.offlineUsers) == 0 {
		return
	}

	check := time.Now().Add(-15 * time.Second)
	for key, offTime := range d.offlineUsers {
		if offTime.Before(check) {
			c, err := d.cDAO.GetByUser(key)
			if err != nil {
				return
			}

			_ = d.callStatusUpdate(&c, data.CallStatusLost)
			d.sendEvent(&c)

			delete(d.offlineUsers, key)
		}
	}
}
func (d *CallService) sendEvent(c *data.Call) {
	msg, _ := json.Marshal(&Call{
		ID:     c.ID,
		Status: c.Status,
		Start:  c.Start,
		Users:  []int{c.FromUserID, c.ToUserID},
	})
	d.hub.Publish("signal", Signal{
		Type:    "connect",
		Message: string(msg),
		Users:   []int{c.FromUserID, c.ToUserID},
	})
}

func (d *CallService) callStatusUpdate(c *data.Call, status int) error {
	err := d.cDAO.Update(c, status)
	if err != nil {
		return err
	}
	if status == data.CallStatusAccepted && c.ChatID != 0{
		err = d.mDAO.Save(&data.Message{
			Text:   "",
			ChatID: c.ChatID,
			UserID: 0,
			Type:   data.CallStartMessage,
		})
		if err != nil {
			return err
		}
	}
	if (status == data.CallStatusEnded || status == data.CallStatusLost) && c.ChatID != 0{
		err = d.mDAO.Save(&data.Message{
			Text:   "",
			ChatID: c.ChatID,
			UserID: 0,
			Type:   data.CallEndMessage,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
