package service

import (
	"mkozhukh/chat/data"

	remote "github.com/mkozhukh/go-remote"
)

type ServiceAll struct {
	Calls         *baseCallService
	GroupCalls    *groupCallService
	PersonalCalls *personalCallService
	Informer      *informerService
	UsersActivity *usersActivityService
	Livekit       *livekitService
	Bots          *botsService
}

func NewService(dao *data.DAO, hub *remote.Hub, livekitConfig LivekitConfig) *ServiceAll {
	s := &ServiceAll{}

	livekit := newLivekitService(livekitConfig)
	baseCall := newCallService(dao, s, livekit != nil)

	s.Livekit = livekit
	s.Calls = &baseCall
	s.GroupCalls = newGroupCallsService(baseCall)
	s.PersonalCalls = newPersonalCallService(baseCall)
	s.UsersActivity = newActivityService(dao, s)
	s.Informer = newInformerService(dao, hub)
	s.Bots = newBotsService(dao)

	CallProvider = &CallServiceProvider{
		group:    s.GroupCalls,
		personal: s.PersonalCalls,
	}

	return s
}
