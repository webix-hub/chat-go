package data

import "github.com/jinzhu/gorm"

type CallUsersDAO struct {
	db *gorm.DB
}

const (
	CallUserStatusDisconnected = 0
	CallUserStatusInitiated    = 1
	CallUserStatusConnecting   = 2
	CallUserStatusActive       = 3
)

type CallUser struct {
	CallID   int `gorm:"primaryKey;autoIncrement:false"`
	UserID   int `gorm:"primaryKey;autoIncrement:false"`
	DeviceID int
	Status   int
}

func NewCallUsersDAO(db *gorm.DB) CallUsersDAO {
	return CallUsersDAO{db}
}

func (cu *CallUsersDAO) AddUser(callId, userId, device int, status int) error {
	cp := CallUser{
		CallID:   callId,
		UserID:   userId,
		DeviceID: device,
		Status:   status,
	}
	err := cu.db.Create(&cp).Error

	return err
}

func (cu *CallUsersDAO) UpdateUserDeviceID(callId, userId, device int, status int) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ? AND user_id = ?", callId, userId).
		Updates(map[string]interface{}{
			"device_id": device,
			"status":    status,
		}).Error

	return err
}

func (cu *CallUsersDAO) UpdateUserConnState(callId, userId int, status int) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ? AND user_id = ?", callId, userId).
		Updates(map[string]interface{}{
			"status": status,
		}).Error

	return err
}

func (cu *CallUsersDAO) GetCallUsers(callId int) ([]CallUser, error) {
	data := []CallUser{}
	err := cu.db.Where("call_id = ?", callId).Find(&data).Error
	return data, err
}

func (cu *CallUsersDAO) GetNotDisconnectedCallUsers(callId int) ([]CallUser, error) {
	data := []CallUser{}
	err := cu.db.Where("call_id = ? && status > ?", callId, CallUserStatusDisconnected).Find(&data).Error
	return data, err
}

func (cu *CallUsersDAO) EndCall(callId int) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ?", callId).
		Updates(map[string]interface{}{
			"status": 0,
		}).Error

	return err
}

func (cu CallUser) TableName() string {
	return "call_user"
}
