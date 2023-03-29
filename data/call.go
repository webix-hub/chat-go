package data

import (
	"errors"
	"time"

	"github.com/jinzhu/gorm"
)

const (
	CallStatusInitiated    = 1
	CallStatusAccepted     = 2
	CallStatusActive       = 3
	CallStatusDisconnected = 801
	CallStatusRejected     = 901
	CallStatusEnded        = 902
	CallStatusIgnored      = 903
	CallStatusLost         = 904
	CallStatusBusy         = 905
)

type CallsDAO struct {
	dao *DAO
	db  *gorm.DB
}

type Call struct {
	ID          int        `gorm:"primary_key"`
	Start       *time.Time `gorm:"column:start"`
	Status      int        `gorm:"column:status"`
	InitiatorID int        `gorm:"column:initiator_id"`
	ChatID      int        `gorm:"column:chat_id"`
	IsGroupCall bool       `gorm:"column:is_group"`
	RoomName    string     `gorm:"column:room_name"`

	Users []CallUser `sql:"-"`
}

func NewCallsDAO(dao *DAO, db *gorm.DB) CallsDAO {
	return CallsDAO{dao: dao, db: db}
}

func (d *CallsDAO) Start(from, device, to, chatId int) (Call, error) {
	// check if initiator is already in call
	check, err := d.checkIfUserInCall(from)
	if err != nil {
		return Call{}, err
	} else if check {
		return Call{}, errors.New("already in the call")
	}

	c := Call{
		InitiatorID: from,
		Status:      CallStatusInitiated,
		IsGroupCall: to == 0,
		ChatID:      chatId,
	}

	if !c.IsGroupCall {
		// check if the user being called is already in a call
		check, err = d.checkIfUserInCall(to)
		if err != nil {
			return Call{}, err
		} else if check {
			c.Status = CallStatusBusy
		}
	}

	err = d.db.Save(&c).Error
	if err != nil || c.Status == CallStatusBusy {
		return c, err
	}

	// add info about the users who can participate in this call
	err = d.setCallUsers(&c, device)

	return c, err
}

func (d *CallsDAO) Get(id int) (Call, error) {
	c := Call{}
	err := d.db.Where("id=?", id).Find(&c).Error
	if err != nil {
		return c, err
	}

	// add info about users who are participating in call
	callUsers, err := d.dao.CallUsers.GetCallUsers(c.ID)
	if err == nil {
		c.Users = callUsers
	}

	return c, err
}

func (d *CallsDAO) GetByUser(id, device int) (Call, error) {
	sql := "SELECT `calls`.* FROM `calls` " +
		"JOIN `call_user` ON `calls`.`id` = `call_user`.`call_id` AND (`call_user`.`user_id` = ? AND (`call_user`.`device_id` = ? OR `call_user`.`device_id` = 0)) " +
		"WHERE `calls`.`status` < 900"

	c := Call{}
	err := d.db.Raw(sql, id, device).Scan(&c).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			err = nil
		} else {
			return Call{}, err
		}
	}

	// add info about users who are participating in call
	callUsers, err := d.dao.CallUsers.GetCallUsers(c.ID)
	if err == nil {
		c.Users = callUsers
	}

	return c, err
}

func (d *CallsDAO) Update(call *Call, status int) error {
	if status == CallStatusAccepted {
		if call.IsGroupCall && call.Status != CallStatusInitiated {
			return nil
		}
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
	sql := "SELECT `calls`.* FROM `calls` " +
		"JOIN `call_user` ON `calls`.`id` = `call_user`.`call_id` AND `call_user`.`connected` = 1 AND `call_user`.`device_id` = ? " +
		"WHERE `calls`.`status` < 900"

	c := Call{}
	err := d.db.Raw(sql, id).Scan(&c).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			err = nil
		} else {
			return Call{}, err
		}
	}

	// add info about users who are participating in call
	callUsers, err := d.dao.CallUsers.GetCallUsers(c.ID)
	if err == nil {
		c.Users = callUsers
	}

	return c, err
}

func (d *CallsDAO) CheckIfChatInCall(chatId int) (Call, error) {
	call := Call{}
	err := d.db.
		Where("chat_id = ? AND (status = ? OR status = ?)", chatId, CallStatusActive, CallStatusInitiated).
		Find(&call).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			err = nil
		}
	}

	// add info about users who are participating in call
	callUsers, err := d.dao.CallUsers.GetCallUsers(call.ID)
	if err == nil {
		call.Users = callUsers
	}

	return call, err
}

func (d *CallsDAO) checkIfUserInCall(uid int) (bool, error) {
	sql := "SELECT `calls`.* FROM `calls` " +
		"JOIN `call_user` ON `calls`.`id` = `call_user`.`call_id` AND `call_user`.`connected` = 1 AND `call_user`.`user_id` = ? " +
		"WHERE `calls`.`status` = ? OR `calls`.`status` = ?"

	check := Call{}
	err := d.db.Raw(sql, uid, CallStatusActive, CallStatusInitiated).Scan(&check).Error
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			err = nil
		}
	}

	return check.ID != 0, err
}

func (d *CallsDAO) setCallUsers(call *Call, initiatorDeviceId int) error {
	chatusers, err := d.dao.UserChats.ByChat(call.ChatID)
	if err != nil {
		return err
	}

	for _, u := range chatusers {
		cu := CallUser{
			CallID: call.ID,
			UserID: u.UserID,
		}
		if u.UserID == call.InitiatorID {
			cu.DeviceID = initiatorDeviceId
			cu.Connected = true
		}
		err := d.addCallUser(call, cu)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *CallsDAO) UpdateCallUsers(call *Call, chatusers []int) ([]CallUser, []CallUser, error) {
	// clear call users
	err := d.dao.CallUsers.db.Delete(&CallUser{}, "call_id = ?", call.ID).Error
	if err != nil {
		return nil, nil, err
	}

	oldCallUsers := make([]CallUser, len(call.Users))
	copy(oldCallUsers, call.Users)
	call.Users = make([]CallUser, 0)

	findUser := func(userId int, users []CallUser) CallUser {
		for i := range users {
			if users[i].UserID == userId {
				return users[i]
			}
		}
		return CallUser{}
	}

	added := make([]CallUser, 0)
	deleted := make([]CallUser, 0)

	for _, userId := range chatusers {
		u := findUser(userId, oldCallUsers)
		if u.UserID == 0 {
			// new user added
			u = CallUser{
				CallID: call.ID,
				UserID: userId,
			}
			added = append(added, u)
		}
		err = d.addCallUser(call, u)
		if err != nil {
			return nil, nil, err
		}
	}

	for _, u := range oldCallUsers {
		check := findUser(u.UserID, call.Users)
		if check.UserID == 0 {
			// deleted user
			deleted = append(deleted, CallUser{UserID: u.UserID})
		}
	}

	return added, deleted, nil
}

func (d *CallsDAO) addCallUser(call *Call, cu CallUser) error {
	// add user to call
	cu.CallID = call.ID

	err := d.dao.CallUsers.AddUser(cu.CallID, cu.UserID, cu.DeviceID, cu.Connected)
	if err == nil {
		call.Users = append(call.Users, cu)
	}

	return err
}

func (c *Call) GetUsersIDs() []int {
	uids := make([]int, len(c.Users))
	for i, u := range c.Users {
		uids[i] = u.UserID
	}
	return uids
}

func (c *Call) GetDevicesIDs() []int {
	devices := make([]int, len(c.Users))
	for i, u := range c.Users {
		devices[i] = u.DeviceID
	}
	return devices
}
