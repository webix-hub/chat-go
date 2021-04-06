package data

import (
	"fmt"
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
	ID         int       `gorm:"primary_key"`
	Start      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	End        time.Time
	Status     int
	FromUserID int `gorm:"column:from"`
	ToUserID   int `gorm:"column:to"`
}

func NewCallsDAO(dao *DAO, db *gorm.DB) CallsDAO {
	return CallsDAO{dao:dao, db:db }
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
	fmt.Println(id)
	err := d.db.Where("(`from`=? or `to`=?) and status < 900", id, id).Find(&c).Error

	return c, err
}

func (d *CallsDAO) Update(call Call, status int) (Call, error) {
	if status == CallStatusAccepted {
		status = CallStatusActive
	}
	if status == CallStatusEnded || status == CallStatusRejected {
		call.End = time.Now()
	}
	call.Status = status

	return call, d.db.Save(&call).Error
}