package data

import "github.com/jinzhu/gorm"

type CallUsersDAO struct {
	db *gorm.DB
}

type CallUser struct {
	CallID    int `gorm:"primaryKey;autoIncrement:false"`
	UserID    int `gorm:"primaryKey;autoIncrement:false"`
	DeviceID  int
	Connected bool
}

func NewCallUsersDAO(db *gorm.DB) CallUsersDAO {
	return CallUsersDAO{db}
}

func (cu *CallUsersDAO) AddUser(callId, userId, device int, connected bool) error {
	cp := CallUser{
		CallID:    callId,
		UserID:    userId,
		DeviceID:  device,
		Connected: connected,
	}
	err := cu.db.Create(&cp).Error

	return err
}

func (cu *CallUsersDAO) UpdateUserDeviceID(callId, userId, device int) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ? AND user_id = ?", callId, userId).
		Updates(map[string]interface{}{
			"device_id": device,
			"connected": true,
		}).Error

	return err
}

func (cu *CallUsersDAO) UpdateUserConnState(callId, userId int, connected bool) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ? AND user_id = ?", callId, userId).
		Updates(map[string]interface{}{
			"connected": connected,
		}).Error

	return err
}

func (cu *CallUsersDAO) GetCallUsers(callId int) ([]CallUser, error) {
	data := []CallUser{}
	err := cu.db.Where("call_id = ?", callId).Find(&data).Error
	return data, err
}

func (cu *CallUsersDAO) EndCall(callId int) error {
	err := cu.db.
		Model(&CallUser{}).
		Where("call_id = ?", callId).
		Updates(map[string]interface{}{
			"connected": false,
		}).Error

	return err
}

func (cu CallUser) TableName() string {
	return "call_user"
}
