package user

import (
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/db"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
)

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
