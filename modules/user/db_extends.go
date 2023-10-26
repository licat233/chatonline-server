package user

import (
	"github.com/gocraft/dbr/v2"
)

// 查询在线的uid有哪些
func (o *onlineDB) QueryOnlineWithUIDs(uids []string) ([]*onlineStatusModel, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	var models []*onlineStatusModel
	_, err := o.session.Select("*").From("user_online").Where("`uid` in ?", uids).Load(&models)
	return models, err
}

// 查询uid是否在线
func (o *onlineDB) QueryOnlineWithUID(uid string) (*onlineStatusModel, error) {
	if len(uid) == 0 {
		return nil, nil
	}
	var model *onlineStatusModel
	_, err := o.session.Select("*").From("user_online").Where("`uid`=?", uid).Load(&model)
	return model, err
}

// Insert 添加用户
func (d *DB) InsertTx(m *Model, tx *dbr.Tx) error {
	return d.insertTx(m, tx)
}

func (d *DB) Tx() *dbr.Tx {
	tx, _ := d.session.Begin()
	return tx
}

func (u *User) DeviceDB() *deviceDB {
	return u.deviceDB
}

func (d *deviceDB) InsertOrUpdateDeviceTx(m *deviceModel, tx *dbr.Tx) error {
	return d.insertOrUpdateDeviceTx(m, tx)
}
