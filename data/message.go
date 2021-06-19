package data

import (
	"time"

	"github.com/jinzhu/gorm"
)

const (
	CallStartMessage    = 900
	CallRejectedMessage = 901
	CallMissedMessage   = 902
)

type MessagesDAO struct {
	dao *DAO
	db  *gorm.DB
}

func NewMessagesDAO(dao *DAO, db *gorm.DB) MessagesDAO {
	return MessagesDAO{dao, db}
}

type Message struct {
	ID     int       `gorm:"primary_key" json:"id"`
	Text   string    `gorm:"type:text" json:"text"`
	Date   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"date"`
	ChatID int       `json:"chat_id"`
	UserID int       `json:"user_id"`
	Type   int       `json:"type"`
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
	err := d.db.Save(&m).Error
	logError(err)

	return err
}

func (d *MessagesDAO) Delete(msgID int) error {
	err := d.db.Delete(&Message{}, msgID).Error

	logError(err)
	return err
}
