package api

import (
	"context"
	"errors"
	"log"
	"net/http"

	remote "github.com/mkozhukh/go-remote"

	"mkozhukh/chat/data"
)

type UserID int
type DeviceID int
type UserList []data.User
type ChatList []data.UserChatDetails

var AccessDeniedError = errors.New("access denied")

type UserEvent struct {
	Op     string      `json:"op"`
	UserID int         `json:"user_id"`
	Data   interface{} `json:"data"`
}

func BuildAPI(db *data.DAO, featuresConfig data.FeaturesConfig, livekitConfig LivekitConfig) *remote.Server {
	if remote.MaxSocketMessageSize < 32000 {
		remote.MaxSocketMessageSize = 32000
	}

	api := remote.NewServer(&remote.ServerConfig{
		WebSocket: true,
	})

	var livekit *LivekitService
	if featuresConfig.WithGroupCalls {
		LIVEKIT_ENABLED = true
		livekit = newLivekitService(livekitConfig)
	}
	service := newCallService(db.Calls, db.CallUsers, db.Messages, db.Chats, db.UserChats, api.Events, livekit)

	api.Events.AddGuard("messages", func(m *remote.Message, c *remote.Client) bool {
		tm, ok := m.Content.(data.MessageEvent)
		if !ok {
			return false
		}
		// operations in user chats, initiated by others
		return int(tm.From) != c.ConnID && db.UsersCache.HasChat(c.User, tm.Msg.ChatID)
	})

	api.Events.AddGuard("chats", func(m *remote.Message, c *remote.Client) bool {
		tm, ok := m.Content.(ChatEvent)
		if !ok {
			return false
		}

		// block if initiated by the same user
		if tm.UserId == c.User {
			return false
		}

		if tm.Users != nil {
			for _, i := range tm.Users {
				if i == c.User {
					return true
				}
			}
		}

		// send message to people in chat only
		return db.UsersCache.HasChat(c.User, tm.ChatID)
	})

	api.Events.AddGuard("signal", func(m *remote.Message, c *remote.Client) bool {
		tm, ok := m.Content.(Signal)
		if !ok {
			return false
		}

		for i := range tm.Users {
			if tm.Users[i] == c.User {
				if tm.Devices[i] == 0 || tm.Devices[i] == c.ConnID {
					return true
				}
			}
		}
		return false
	})

	api.Events.UserHandler = func(u *remote.UserChange) {
		status := data.StatusOffline
		if u.Status {
			status = data.StatusOnline
		}
		go (func() {
			api.Events.Publish("users", UserEvent{Op: "online", UserID: u.ID, Data: status})
			db.Users.ChangeOnlineStatus(u.ID, status)
		})()
	}

	api.Events.ConnHandler = func(u *remote.UserChange) {
		status := data.StatusOffline
		if u.Status {
			status = data.StatusOnline
		}
		go service.ChangeOnlineStatus(u.Connection, status)
	}

	api.Connect = func(r *http.Request) (context.Context, error) {
		id, _ := r.Context().Value("user_id").(int)
		if id == 0 {
			return nil, errors.New("access denied")
		}
		device, _ := r.Context().Value("device_id").(int)
		if device == 0 {
			return nil, errors.New("access denied")
		}

		return context.WithValue(
			context.WithValue(r.Context(), remote.UserValue, id),
			remote.ConnectionValue, device), nil
	}

	must(api.AddService("message", &MessagesAPI{db, featuresConfig}))
	must(api.AddService("chat", &ChatsAPI{db}))
	must(api.AddService("call", &CallsAPI{db, service, livekit}))

	// provide user's id
	must(api.AddVariable("user", UserID(0)))
	must(api.AddVariable("device", DeviceID(0)))
	// after chat initialization, user will always need this info
	// so instead of call waiting, provide it from the start
	must(api.AddVariable("users", UserList{}))
	must(api.AddVariable("chats", ChatList{}))
	must(api.AddVariable("call", Call{}))

	handleDependencies(api, db)
	return api
}

func handleDependencies(api *remote.Server, db *data.DAO) {
	must(api.Dependencies.AddProvider(func(ctx context.Context) UserID {
		id, _ := ctx.Value("user_id").(int)
		return UserID(id)
	}))
	must(api.Dependencies.AddProvider(func(ctx context.Context) DeviceID {
		id, _ := ctx.Value("device_id").(int)
		return DeviceID(id)
	}))
	must(api.Dependencies.AddProvider(func(ctx context.Context) ChatList {
		id, _ := ctx.Value("user_id").(int)
		u, _ := db.UserChats.GetAll(id)
		return u
	}))
	must(api.Dependencies.AddProvider(func(ctx context.Context) UserList {
		u, _ := db.Users.GetAll()
		return u
	}))
	must(api.Dependencies.AddProvider(func(ctx context.Context) *remote.Hub {
		return api.Events
	}))
	must(api.Dependencies.AddProvider(func(ctx context.Context) Call {
		id, _ := ctx.Value("user_id").(int)
		device, _ := ctx.Value("device_id").(int)
		call, _ := db.Calls.GetByUser(id, device)

		var callName, callAvatar string
		if call.IsGroupCall {
			// if user disconnected from the call, then do not show it to him again
			// but he can reconnect to this call manually (by clicking "Start call" button on the client side)
			for _, cu := range call.Users {
				if cu.UserID == id && cu.DeviceID != 0 && !cu.Connected {
					return Call{}
				}
			}

			chat, _ := db.Chats.GetOne(call.ChatID)
			callName = chat.Name
			callAvatar = chat.Avatar
		}

		return Call{
			ID:          call.ID,
			Name:        callName,
			Avatar:      callAvatar,
			Status:      call.Status,
			Start:       call.Start,
			InitiatorID: call.InitiatorID,
			IsGroupCall: call.IsGroupCall,
			ChatID:      call.ChatID,
			Users:       call.GetUsersIDs(),
		}
	}))
}

func must(err error) {
	if err != nil {
		log.Fatal("can't init remote API\n", err.Error())
	}
}
