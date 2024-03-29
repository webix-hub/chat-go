package data

import (
	"time"

	"github.com/jinzhu/gorm"
)

const (
	CallStartMessage    = 900
	CallRejectedMessage = 901
	CallMissedMessage   = 902
	CallBusyMessage     = 903
	AttachedFile        = 800
	VoiceMessage        = 801
	BotMessage          = 700
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
	ID        int              `gorm:"primary_key" json:"id"`
	Text      string           `gorm:"type:text" json:"text"`
	Date      time.Time        `gorm:"default:CURRENT_TIMESTAMP" json:"date"`
	Edited    bool             `json:"edited"`
	ChatID    int              `json:"chat_id"`
	UserID    int              `json:"user_id"`
	Type      int              `json:"type"`
	Related   int              `json:"-"`
	Reactions map[string][]int `sql:"-" json:"reactions"`
}

func (d *MessagesDAO) GetOne(msgID int) (*Message, error) {
	t := Message{}
	err := d.db.Where("id = ?", msgID).First(&t).Error
	if err != nil {
		logError(err)
		return nil, err
	}

	if Features.WithReactions {
		t.Reactions, err = d.dao.Reactions.GetAllForMessage(msgID)
	}

	return &t, err
}

func (d *MessagesDAO) GetLast(chatId int) (*Message, error) {
	t := Message{}
	err := d.db.Where("chat_id = ?", chatId).Order("date desc").Last(&t).Error
	if err != nil {
		logError(err)
		return nil, err
	}

	if Features.WithReactions {
		t.Reactions, err = d.dao.Reactions.GetAllForMessage(t.ID)
		logError(err)
	}

	return &t, err
}

func (d *MessagesDAO) GetLastN(chatId, count int) ([]Message, error) {
	msgs := make([]Message, 0, count)
	err := d.db.Where("chat_id = ?", chatId).Order("date desc").Limit(count).Find(&msgs).Error
	if err != nil {
		logError(err)
		return nil, err
	}

	return msgs, err
}

func (d *MessagesDAO) GetAll(chatID int) ([]Message, error) {
	msgs := make([]Message, 0)
	err := d.db.Where("chat_id = ?", chatID).Order("date ASC").Find(&msgs).Error
	if err != nil {
		logError(err)
		return nil, err
	}

	if Features.WithReactions {
		reactions, err := d.dao.Reactions.GetAllForChat(chatID)
		if err != nil {
			return nil, err
		}

		d.dao.Reactions.SetReactions(msgs, reactions)
	}

	return msgs, err
}

func (d *MessagesDAO) Save(m *Message) error {
	err := d.db.Save(&m).Error
	logError(err)

	return err
}

func (d *MessagesDAO) Delete(msgID int) error {
	err := d.db.Delete(&Message{}, msgID).Error
	if err != nil {
		return err
	}

	err = d.db.Where("message_id = ?", msgID).Delete(&Reaction{}).Error

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

func (d *MessagesDAO) Append(id int, text string, final bool, userId int) error {
	msg, err := d.GetOne(id)

	if err != nil {
		return err
	}
	if msg.UserID != int(userId) {
		return ErrAccessDenied
	}

	text = SafeHTML(text)
	prevText := msg.Text

	msg.Text += text
	err = d.Save(msg)
	if err != nil {
		return err
	}

	msg.Text = text
	d.dao.Hub.Publish("messages", MessageEvent{Op: "append", Msg: msg, From: 0})

	if prevText == "" {
		_, err = d.dao.Chats.SetLastMessage(msg.ChatID, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *MessagesDAO) Send(c int, msg *Message, origin string, from int) error {
	d.dao.Hub.Publish("messages", MessageEvent{Op: "add", Msg: msg, Origin: origin, From: from})

	err := d.dao.UserChats.IncrementCounter(c, msg.UserID)
	if err != nil {
		return err
	}

	if msg.Text != "" {
		_, err = d.dao.Chats.SetLastMessage(c, msg)
		if err != nil {
			return err
		}
	}

	return nil
}
