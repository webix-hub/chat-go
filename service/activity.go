package service

import (
	"mkozhukh/chat/data"
	"time"
)

type usersActivityService struct {
	dao *data.DAO
	all *ServiceAll

	offlineDevices map[int]time.Time
}

func newActivityService(dao *data.DAO, all *ServiceAll) *usersActivityService {
	service := usersActivityService{
		dao:            dao,
		offlineDevices: make(map[int]time.Time),
		all:            all,
	}
	go service.runCheckOfflineUsers()
	return &service
}

func (s *usersActivityService) ChangeOnlineStatus(device int, status int) {
	if status == data.StatusOnline {
		delete(s.offlineDevices, device)
		return
	}

	if status == data.StatusOffline {
		s.offlineDevices[device] = time.Now()
	}
}

func (s *usersActivityService) checkOfflineUsers() {
	if len(s.offlineDevices) == 0 {
		return
	}

	check := time.Now().Add(-15 * time.Second)
	for key, offTime := range s.offlineDevices {
		if offTime.Before(check) {
			c, err := s.dao.Calls.GetByDevice(key)
			if err != nil {
				return
			}

			if !c.IsGroupCall {
				// drop the call
				s.all.Calls.updateStatusAndSendMessage(&c, data.CallStatusLost)
				s.all.Informer.SendSignalToCall(&c, data.CallStatusLost)
			} else {
				for _, u := range c.Users {
					if u.DeviceID == key {
						// drop the call
						s.dao.CallUsers.UpdateUserConnState(c.ID, u.UserID, data.CallUserStatusDisconnected)
						s.all.Informer.SendSignalToCall(&c, data.CallStatusLost, u)
						break
					}
				}
			}

			delete(s.offlineDevices, key)
		}
	}
}

func (s *usersActivityService) runCheckOfflineUsers() {
	for range time.Tick(time.Second * 10) {
		s.checkOfflineUsers()
	}
}
