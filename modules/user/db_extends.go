package user

import (
	"time"

	"github.com/gocraft/dbr/v2"
)

// 查询uids的在线信息
func (o *onlineDB) QueryOnlineWithUIDs(uids []string) ([]*onlineStatusModel, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	var models []*onlineStatusModel
	_, err := o.session.Select("*").From("user_online").Where("`uid` in ?", uids).Load(&models)
	return models, err
}

// 查询uid的在线信息
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

func (u *User) OnlineDB() *onlineDB {
	return u.onlineDB
}

func (o *onlineStatusModel) IsOnline() bool {
	if o.Online == 1 {
		return true
	}
	t := time.Unix(int64(o.LastOffline), 0)
	//离线时间小于10分钟
	return time.Since(t).Minutes() < 10
}

// 取出客服
func (d *DB) QueryKefuByUIDs(uids []string) ([]*Model, error) {
	if len(uids) <= 0 {
		return nil, nil
	}
	var models []*Model
	_, err := d.session.Select("*").From("user").Where("uid in ?", uids).Where("(category='service' OR category='') and status=1").Load(&models)
	return models, err
}
