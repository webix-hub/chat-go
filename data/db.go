package data

import (
	"log"
	"strings"

	remote "github.com/mkozhukh/go-remote"

	"github.com/jinzhu/gorm"
)

var Debug = 1
var Features FeaturesConfig

func logError(e error) {
	if e != nil && Debug > 0 {
		log.Printf("[ERROR]\n%s\n", e)
	}
}

func SafeHTML(in string) string {
	return strings.ReplaceAll(in, "<", "&lt;")
}

func SafeUrl(in string) string {
	return strings.ReplaceAll(in, "\"", "")
}

type DAO struct {
	db *gorm.DB

	Users     UsersDAO
	Messages  MessagesDAO
	UserChats UserChatsDAO
	Chats     ChatsDAO
	Calls     CallsDAO
	CallUsers CallUsersDAO
	Files     FilesDAO
	Reactions ReactionsDAO

	Hub        *remote.Hub
	UsersCache UsersCache
}

func (d *DAO) GetDB() *gorm.DB {
	return d.db
}

func NewDAO(db *gorm.DB, config FeaturesConfig) *DAO {
	d := DAO{}

	if Debug > 1 {
		db.LogMode(true)
	}

	Features = config

	d.db = db
	d.Users = NewUsersDAO(&d, db)
	d.Chats = NewChatsDAO(&d, db)
	d.Messages = NewMessagesDAO(&d, db)
	d.UserChats = NewUserChatsDAO(&d, db)
	d.Calls = NewCallsDAO(&d, db)
	d.CallUsers = NewCallUsersDAO(db)
	d.Files = NewFilesDAO(&d, db)
	d.Reactions = NewReactionDAO(&d, db)

	d.UsersCache = NewUsersCache(&d)

	d.db.AutoMigrate(&User{})
	d.db.AutoMigrate(&Message{})
	d.db.AutoMigrate(&Chat{}, &UserChat{})
	d.db.AutoMigrate(&Call{})
	d.db.AutoMigrate(&CallUser{})
	d.db.AutoMigrate(&File{})
	d.db.AutoMigrate(&Reaction{})

	return &d
}

func (d *DAO) SetHub(r *remote.Hub) {
	d.Hub = r
}
