package data

import (
	"errors"

	"github.com/jinzhu/gorm"
)

type ReactionsDAO struct {
	dao *DAO
	db  *gorm.DB
}

type Reaction struct {
	Id        int    `gorm:"primary_key" json:"reaction_id"`
	MessageId int    `json:"message_id"`
	Reaction  string `json:"reaction"`
	UserId    int    `json:"user_id"`
}

var getReactionsSql = "select r.id, r.message_id, r.reaction, r.user_id from messages m join reactions r on m.id = r.message_id and m.chat_id = ?"

func NewReactionDAO(dao *DAO, db *gorm.DB) ReactionsDAO {
	return ReactionsDAO{dao, db}
}

func (d *ReactionsDAO) Add(reaction Reaction) (bool, error) {
	if d.Exists(reaction) {
		return false, errors.New("record already exists")
	}

	res := d.db.Save(&reaction)
	logError(res.Error)

	return res.RowsAffected != 0, res.Error
}

func (d *ReactionsDAO) Remove(reaction Reaction) error {
	err := d.db.Where(
		"message_id = ? and reaction = ? and user_id = ?",
		reaction.MessageId,
		reaction.Reaction,
		reaction.UserId,
	).Delete(&reaction).Error
	logError(err)

	return err
}

func (d *ReactionsDAO) GetAllForChat(chatId int) ([]Reaction, error) {
	reactions := make([]Reaction, 0)
	err := d.db.Raw(getReactionsSql, chatId).Scan(&reactions).Error
	logError(err)

	return reactions, err
}

func (d *ReactionsDAO) GetAllForMessage(msgId int) (map[string][]int, error) {
	reactions := make([]Reaction, 0)
	err := d.db.Where("message_id = ?", msgId).Find(&reactions).Error
	logError(err)

	return d.ToMap(reactions), err
}

func (d *ReactionsDAO) ToMap(reactions []Reaction) map[string][]int {
	res := make(map[string][]int)
	for _, r := range reactions {
		res[r.Reaction] = append(res[r.Reaction], r.UserId)
	}

	return res
}

func (d *ReactionsDAO) SetReactions(msgs []Message, all []Reaction) {
	for i := range msgs {
		msgs[i].Reactions = getReactionsForMessage(msgs[i].ID, all)
	}
}

func (d *ReactionsDAO) Exists(reaction Reaction) bool {
	r := Reaction{}
	err := d.db.Where(
		"message_id = ? and reaction = ? and user_id = ?",
		reaction.MessageId,
		reaction.Reaction,
		reaction.UserId,
	).Take(&r).Error

	return err == nil
}

func getReactionsForMessage(msgId int, all []Reaction) map[string][]int {
	reactions := make(map[string][]int)
	for _, r := range all {
		if r.MessageId == msgId {
			reactions[r.Reaction] = append(reactions[r.Reaction], r.UserId)
		}
	}

	return reactions
}
