package api

import (
	"context"
	"errors"
	"log"

	remote "github.com/mkozhukh/go-remote"

	"mkozhukh/chat/data"
)

type UserID int
type UserList []data.User
type ChatList []data.UserChatDetails

var AccessDeniedError = errors.New("access denied")

type UserEvent struct {
	Op     string      `json:"op"`
	UserID int         `json:"user_id"`
	Data   interface{} `json:"data"`
}

func BuildAPI(db *data.DAO) *remote.Server {
	api := remote.NewServer(&remote.ServerConfig{
		WebSocket: true,
	})

	api.Events.AddGuard("messages", func(m *remote.Message, c *remote.Client) bool {
		tm, ok := m.Content.(MessageEvent)
		if !ok {
			return false
		}

		// operations in user chats, initiated by others
		return tm.Msg.UserID != c.User && db.UsersCache.HasChat(c.User, tm.Msg.ChatID)
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

	api.Events.UserHandler = func(u *remote.UserChange) {
		status := data.StatusOffline
		if u.Status {
			status = data.StatusOnline
		}

		go changeOnlineStatus(db, api.Events, u.ID, status)
	}

	api.Connect = func(ctx context.Context) (context.Context, error) {
		id, _ := ctx.Value("user_id").(int)
		if id == 0 {
			return nil, errors.New("access denied")
		}

		return context.WithValue(ctx, remote.UserValue, id), nil
	}

	must(api.AddService("message", &MessagesAPI{db}))
	must(api.AddService("chat", &ChatsAPI{db}))

	// provide user's id
	must(api.AddVariable("user", UserID(0)))
	// after chat initialization, user will always need this info
	// so instead of call waiting, provide it from the start
	must(api.AddVariable("users", UserList{}))
	must(api.AddVariable("chats", ChatList{}))

	handleDependencies(api, db)
	return api
}

func changeOnlineStatus(dao *data.DAO, events *remote.Hub, user int, status int) {
	events.Publish("users", UserEvent{Op: "online", UserID: user, Data: status})
	dao.Users.ChangeOnlineStatus(user, status)
}

func handleDependencies(api *remote.Server, db *data.DAO) {
	must(api.Dependencies.AddProvider(func(ctx context.Context) UserID {
		id, _ := ctx.Value("user_id").(int)
		return UserID(id)
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
}

func must(err error) {
	if err != nil {
		log.Fatal("can't init remote API\n", err.Error())
	}
}
