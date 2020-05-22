package data

import (
	"time"

	"github.com/jinzhu/gorm"
)

type UserChatsDAO struct {
	dao *DAO
	db  *gorm.DB
}

func NewUserChatsDAO(dao *DAO, db *gorm.DB) UserChatsDAO {
	return UserChatsDAO{dao, db}
}

const (
	ChatStatusNormal int = iota + 1
	ChatStatusFavorite
	ChatStatusHidden
)

type UserChat struct {
	ID          int `gorm:"primary_key" json:"id"`
	ChatID      int `json:"chat_id"`
	UserID      int `json:"user_id"`
	UnreadCount int `json:"unread_count"`
	DirectID    int `json:"direct_id"`
	Status      int `json:"status"`
}

type UserChatDetails struct {
	UserChat
	Name    string     `json:"name"`
	Date    *time.Time `json:"date"`
	Message string     `json:"message"`
	Users   []int      `json:"users"`
	Avatar  string     `json:"avatar"`
}

var getUserChatsSQL = "select chats.id, chats.name, chats.avatar, " +
	"user_chats.direct_id, user_chats.status, user_chats.unread_count, " +
	"messages.text as message, messages.date " +
	"from user_chats " +
	"inner join chats on user_chats.chat_id = chats.id " +
	"left outer join messages on chats.last_message = messages.id " +
	"where user_chats.user_id = ? " +
	"order by messages.date desc"

var getUserChatSQL = "select chats.id, chats.name, chats.avatar, " +
	"user_chats.direct_id, user_chats.status, user_chats.unread_count, " +
	"messages.text as message, messages.date " +
	"from user_chats " +
	"inner join chats on user_chats.chat_id = chats.id " +
	"left outer join messages on chats.last_message = messages.id " +
	"where user_chats.chat_id = ? AND user_chats.user_id = ? " +
	"order by messages.date desc"

var getUserChatLeaveSQL = "select chats.id, chats.name, chats.avatar, " +
	"messages.text as message, messages.date " +
	"from chats " +
	"left outer join messages on chats.last_message = messages.id " +
	"where chats.id = ?"

func (d *UserChatsDAO) GetAll(userId int) ([]UserChatDetails, error) {
	uc := make([]UserChatDetails, 0)
	err := d.db.Raw(getUserChatsSQL, userId).Scan(&uc).Error
	logError(err)
	if err != nil {
		return nil, err
	}

	for i := range uc {
		// [FIXME] is it safe to return mutable arrays from user cache ?
		uc[i].Users = d.dao.UsersCache.GetUsers(uc[i].ID)
	}
	return uc, nil
}

func (d *UserChatsDAO) GetOne(chatId, userId int) (*UserChatDetails, error) {
	uc := UserChatDetails{}
	var err error

	err = d.db.Raw(getUserChatSQL, chatId, userId).Scan(&uc).Error
	logError(err)
	if err != nil {
		return nil, err
	}

	// [FIXME] is it safe to return mutable arrays from user cache ?
	uc.Users = d.dao.UsersCache.GetUsers(chatId)

	return &uc, nil
}

func (d *UserChatsDAO) GetOneLeaved(chatId int) (*UserChatDetails, error) {
	uc := UserChatDetails{}
	var err error

	err = d.db.Raw(getUserChatLeaveSQL, chatId).Scan(&uc).Error
	logError(err)
	if err != nil {
		return nil, err
	}

	// [FIXME] is it safe to return mutable arrays from user cache ?
	uc.Users = d.dao.UsersCache.GetUsers(chatId)

	return &uc, nil
}

func (d *UserChatsDAO) ByUser(userId int) ([]UserChat, error) {
	userChats := make([]UserChat, 0)
	err := d.db.Where("user_id = ?", userId).Find(&userChats).Error
	logError(err)

	return userChats, err
}

func (d *UserChatsDAO) ResetCounter(chatId int, userId int) error {
	err := d.db.Table("user_chats").
		Where("chat_id = ? and user_id = ?", chatId, userId).
		Update("unread_count", 0).Error
	logError(err)

	return err
}

func (d *UserChatsDAO) ByChat(chatId int) ([]UserChat, error) {
	userChat := make([]UserChat, 0)
	err := d.db.Where("chat_id = ?", chatId).Find(&userChat).Error

	logError(err)
	return userChat, err
}

func (d *UserChatsDAO) IncrementCounter(chatId, userId int) error {
	err := d.db.Table("user_chats").
		Where("chat_id = ? AND user_id <> ?", chatId, userId).
		Update("unread_count", gorm.Expr("unread_count + ?", 1)).Error
	logError(err)

	return err
}
