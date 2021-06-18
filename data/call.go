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
	ID           int        `gorm:"primary_key"`
	Start        *time.Time `gorm:"column:start"`
	Status       int        `gorm:"column:status"`
	FromUserID   int        `gorm:"column:from"`
	ToUserID     int        `gorm:"column:to"`
	FromDeviceID int        `gorm:"column:from_device"`
	ToDeviceID   int        `gorm:"column:to_device"`
	ChatID       int        `gorm:"column:chat_id"`
}

func NewCallsDAO(dao *DAO, db *gorm.DB) CallsDAO {
	return CallsDAO{dao: dao, db: db}
}

func (d *CallsDAO) Start(from, device, to, chatId int) (Call, error) {
	c := Call{
		FromUserID:   from,
		FromDeviceID: device,
		ToUserID:     to,
		ToDeviceID:   0,
		Status:       CallStatusInitiated,
		ChatID:       chatId,
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

func (d *CallsDAO) GetByUser(id, device int) (Call, error) {
	c := Call{}
	err := d.db.Where("((`from`=? and (`from_device` = ? or `from_device` = 0)) or (`to`=? and (`to_device` = ? or `to_device` = 0))) and status < 900", id, device, id, device).Find(&c).Error

	return c, err
}

func (d *CallsDAO) Update(call *Call, status int) error {
	if status == CallStatusAccepted {
		status = CallStatusActive
		currentTime := time.Now()
		call.Start = &currentTime
	}

	call.Status = status
	return d.db.Save(&call).Error
}

func (d *CallsDAO) Save(call *Call) error {
	return d.db.Save(&call).Error
}

func (d *CallsDAO) GetByDevice(id int) (Call, error) {
	c := Call{}
	err := d.db.Where("(`from_device`=? or `to_device` = ?) and status < 900", id, id).Find(&c).Error

	return c, err
}
