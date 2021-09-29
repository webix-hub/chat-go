package data

import (
	"time"

	"github.com/jinzhu/gorm"
)

const (
	CallStartMessage    = 900
	CallRejectedMessage = 901
	CallMissedMessage   = 902
	AttachedFile        = 800
)

type MessagesDAO struct {
	dao *DAO
	db  *gorm.DB
}

type MessageEvent struct {
	Op     string   `json:"op"`
	Msg    *Message `json:"msg"`
	Origin string   `json:"origin,omitempty"`
	From   int
}

func NewMessagesDAO(dao *DAO, db *gorm.DB) MessagesDAO {
	return MessagesDAO{dao, db}
}

type Message struct {
	ID      int       `gorm:"primary_key" json:"id"`
	Text    string    `gorm:"type:text" json:"text"`
	Date    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"date"`
	ChatID  int       `json:"chat_id"`
	UserID  int       `json:"user_id"`
	Type    int       `json:"type"`
	Related int       `json:"-"`
}

func (d *MessagesDAO) GetOne(msgID int) (*Message, error) {
	t := Message{}
	err := d.db.Where("id = ?", msgID).First(&t).Error

	logError(err)
	return &t, err
}

func (d *MessagesDAO) GetLast(chatId int) (*Message, error) {
	t := Message{}
	err := d.db.Where("chat_id=?", chatId).Order("date desc").Last(&t).Error
	logError(err)

	return &t, err
}

func (d *MessagesDAO) GetAll(chatID int) ([]Message, error) {
	msgs := make([]Message, 0)

	err := d.db.Where("chat_id = ?", chatID).Order("date ASC").Find(&msgs).Error

	logError(err)
	return msgs, err
}

func (d *MessagesDAO) Save(m *Message) error {
	m.Date = time.Now()
	err := d.db.Save(&m).Error
	logError(err)

	return err
}

func (d *MessagesDAO) Delete(msgID int) error {
	err := d.db.Delete(&Message{}, msgID).Error

	logError(err)
	return err
}

func (d *MessagesDAO) SaveAndSend(c int, msg *Message, origin string, from int) error {
	err := d.Save(msg)
	if err != nil {
		return err
	}

	return d.Send(c, msg, origin, from)
}

func (d *MessagesDAO) Send(c int, msg *Message, origin string, from int) error {
	d.dao.Hub.Publish("messages", MessageEvent{Op: "add", Msg: msg, Origin: origin, From: from})

	err := d.dao.UserChats.IncrementCounter(c, msg.UserID)
	if err != nil {
		return err
	}

	_, err = d.dao.Chats.SetLastMessage(c, msg)
	if err != nil {
		return err
	}

	return nil
}
