package data

import (
	remote "github.com/mkozhukh/go-remote"
	"log"

	"github.com/jinzhu/gorm"
)

var Debug = 1

func logError(e error) {
	if e != nil && Debug > 0 {
		log.Printf("[ERROR]\n%s\n", e)
	}
}

type DAO struct {
	db *gorm.DB

	Users     UsersDAO
	Messages  MessagesDAO
	UserChats UserChatsDAO
	Chats     ChatsDAO
	Calls     CallsDAO
	Files     FilesDAO

	Hub        *remote.Hub
	UsersCache UsersCache
}

func (d *DAO) GetDB() *gorm.DB {
	return d.db
}

func NewDAO(db *gorm.DB) *DAO {
	d := DAO{}

	if Debug > 1 {
		db.LogMode(true)
	}

	d.db = db
	d.Users = NewUsersDAO(&d, db)
	d.Chats = NewChatsDAO(&d, db)
	d.Messages = NewMessagesDAO(&d, db)
	d.UserChats = NewUserChatsDAO(&d, db)
	d.Calls = NewCallsDAO(&d, db)
	d.Files = NewFilesDAO(&d, db)

	d.UsersCache = NewUsersCache(&d)

	d.db.AutoMigrate(&User{})
	d.db.AutoMigrate(&Message{})
	d.db.AutoMigrate(&Chat{}, &UserChat{})
	d.db.AutoMigrate(&Call{})
	d.db.AutoMigrate(&File{})

	return &d
}

func (d *DAO) SetHub(r *remote.Hub) {
	d.Hub = r
}
