package data

import (
	"github.com/jinzhu/gorm"
	"time"
)

const (
	CallStatusInitiated = 1
	CallStatusAccepted  = 2
	CallStatusActive    = 3
	CallStatusRejected  = 901
	CallStatusEnded     = 902
	CallStatusIgnored   = 903
	CallStatusLost      = 904
)

type CallsDAO struct {
	dao *DAO
	db  *gorm.DB
}

type Call struct {
	ID         int        `gorm:"primary_key"`
	Start      *time.Time `gorm:"column:start"`
	Status     int        `gorm:"column:status"`
	FromUserID int        `gorm:"column:from"`
	ToUserID   int        `gorm:"column:to"`
	ChatID     int        `gorm:"column:chat_id"`
}

func NewCallsDAO(dao *DAO, db *gorm.DB) CallsDAO {
	return CallsDAO{dao: dao, db: db}
}

func (d *CallsDAO) Start(from, to int) (Call, error) {
	c := Call{
		FromUserID: from,
		ToUserID:   to,
		Status:     CallStatusInitiated,
	}

	err := d.db.Save(&c).Error
	if err != nil {
		return c, err
	}

	return c, err
}

func (d *CallsDAO) Get(id int) (Call, error) {
	c := Call{}
	err := d.db.Where("id=?", id).Find(&c).Error

	return c, err
}

func (d *CallsDAO) GetByUser(id int) (Call, error) {
	c := Call{}
	err := d.db.Where("(`from`=? or `to`=?) and status < 900", id, id).Find(&c).Error

	return c, err
}

func (d *CallsDAO) Update(call *Call, status int) error {
	if status == CallStatusAccepted {
		status = CallStatusActive
	}

	currentTime := time.Now()
	call.Start = &currentTime
	call.Status = status

	return d.db.Save(&call).Error
}
