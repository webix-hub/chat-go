package data

import "github.com/jinzhu/gorm"

const (
	StatusOffline int = iota + 1
	StatusOnline
)

type UsersDAO struct {
	dao *DAO
	db  *gorm.DB
}

func NewUsersDAO(dao *DAO, db *gorm.DB) UsersDAO {
	return UsersDAO{dao, db}
}

type User struct {
	ID     uint   `gorm:"primary_key" json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
	UID    string `json:"-"`
	Status int    `json:"status"`
}

func (d *UsersDAO) GetOne(id int) (*User, error) {
	t := User{}
	err := d.db.First(&t, id).Error

	logError(err)
	return &t, err
}

func (d *UsersDAO) GetAll() ([]User, error) {
	t := make([]User, 0)
	err := d.db.Find(&t).Error

	logError(err)
	return t, err
}

func (d *UsersDAO) GetGroupName(users []int) string {
	t := make([]User, 0)
	err := d.db.Find(&t, "id in(?)", users).Error
	logError(err)

	out := ""
	for i, u := range t {
		if i > 0 {
			out += ", "
		}
		out += u.Name
	}

	return out
}

func (d *UsersDAO) ChangeOnlineStatus(id int, status int) {
	u, _ := d.GetOne(id)
	if u.ID == 0 {
		return
	}

	err := d.db.Model(u).Update("status", status).Error
	logError(err)
}
