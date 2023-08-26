package user

import (
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/db"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
)

func (o *OnlineService) QueryOnlineWithUIDs(uids []string) ([]*onlineStatusModel, error) {
	return o.onlineDB.QueryOnlineWithUIDs(uids)
}

func (o *OnlineService) QueryOnlineWithUID(uid string) (*onlineStatusModel, error) {
	return o.onlineDB.QueryOnlineWithUID(uid)
}

func NewVisitorDeviceModel() *deviceModel {
	return &deviceModel{
		UID:         "",
		DeviceID:    util.GenerUUID(),
		DeviceName:  "browser",
		DeviceModel: "",
		LastLogin:   time.Now().Local().Unix(),
		BaseModel: db.BaseModel{
			Id:        0,
			CreatedAt: db.Time{},
			UpdatedAt: db.Time{},
		},
	}
}
