package data

import (
	"github.com/jinzhu/gorm"
)

type ChatsDAO struct {
	dao *DAO
	db  *gorm.DB
}

func NewChatsDAO(dao *DAO, db *gorm.DB) ChatsDAO {
	return ChatsDAO{dao, db}
}

type Chat struct {
	ID          int    `gorm:"primary_key" json:"id"`
	Name        string `json:"name"`
	LastMessage int    `json:"last"`
	Avatar      string `json:"avatar"`
}

func (d *ChatsDAO) GetOne(id int) (*Chat, error) {
	t := Chat{}
	err := d.db.First(&t, id).Error

	logError(err)
	return &t, err
}

func (d *ChatsDAO) Save(c *Chat) error {
	err := d.db.Save(&c).Error
	logError(err)

	return err
}

func (d *ChatsDAO) AddDirect(targetUserId int, userId int) (int, error) {
	userChat := UserChat{}
	err := d.db.Model(&UserChat{}).
		Where("user_id = ? AND direct_id = ?", userId, targetUserId).
		First(&userChat).
		Error
	logError(err)

	// already hae a direct chat
	if userChat.ID != 0 {
		return userChat.ChatID, nil
	}

	chat := Chat{}
	err = d.db.Save(&chat).Error
	logError(err)
	if err != nil {
		return 0, err
	}

	err = d.setUsersToDB(chat.ID, []int{userId}, targetUserId)
	if err == nil {
		err = d.setUsersToDB(chat.ID, []int{targetUserId}, userId)
	}

	return chat.ID, err
}

func (d *ChatsDAO) AddGroup(name, avatar string, users []int) (int, error) {
	chat := Chat{Name: name, Avatar: avatar}
	err := d.db.Save(&chat).Error
	logError(err)
	if err != nil {
		return 0, err
	}

	return chat.ID, d.setUsersToDB(chat.ID, users, 0)
}

func (d *ChatsDAO) SetUsers(chatId int, users []int) (int, error) {
	uChat := UserChat{}
	err := d.db.Where("chat_id = ?", chatId).First(&uChat).Error
	logError(err)
	if err != nil {
		return chatId, err
	}

	if uChat.DirectID > 0 {
		// when adding people to private chate - create new group chat
		name := d.dao.Users.GetGroupName(users)
		chatId, err = d.dao.Chats.AddGroup(name, "", users)
	} else {
		err = d.setUsersToDB(chatId, users, 0)
	}

	return chatId, err
}

func (d *ChatsDAO) Leave(chatId int, userId int) error {
	err := d.leaveChat(chatId, userId)
	if err != nil {
		return err
	}

	// check is that was the last user
	var ccount int
	err = d.db.Table("user_chats").
		Where("chat_id = ?", chatId).
		Count(&ccount).
		Error
	logError(err)
	if err != nil {
		return err
	}

	// delete chat where no more users
	if ccount > 0 {
		return nil
	}

	err = d.db.Table("chats").
		Delete("id = ?", chatId).
		Error
	logError(err)

	return err
}

func (d *ChatsDAO) SetLastMessage(chatId int, msg *Message) (*Message, error) {
	var err error
	if msg == nil {
		msg, err = d.dao.Messages.GetLast(chatId)
		if err != nil {
			return nil, err
		}
	}
	err = d.db.Table("chats").
		Where("id = ?", chatId).
		Update("last_message", msg.ID).
		Error
	logError(err)

	return msg, err
}

func (d *ChatsDAO) setUsersToDB(chat int, next []int, direct int) error {
	for _, u := range next {
		if !d.dao.UsersCache.HasChat(u, chat) {

			err := d.db.Save(&UserChat{
				ChatID:   chat,
				DirectID: direct,
				UserID:   u, // [FIXME] Need to ensure that ID is valid
			}).Error
			logError(err)

			if err != nil {
				return err
			}

			d.dao.UsersCache.JoinChat(u, chat)
		}
	}

	if direct == 0 {
		users := d.dao.UsersCache.GetUsers(chat)
		for _, u := range users {
			found := false
			for _, n := range next {
				if n == u {
					found = true
					break
				}
			}

			if !found {
				err := d.leaveChat(chat, u)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (d *ChatsDAO) leaveChat(chatId, userId int) error {
	err := d.db.Delete(UserChat{}, "chat_id = ? AND user_id = ? AND direct_id = 0", chatId, userId).
		Error
	logError(err)

	if err == nil {
		d.dao.UsersCache.LeaveChat(userId, chatId)
	}
	return err
}

func (d *ChatsDAO) Update(id int, name string, avatar string) error {
	err := d.db.Exec("UPDATE chats SET name = ?, avatar = ? WHERE id = ?", name, avatar, id).Error
	logError(err)
	return err
}