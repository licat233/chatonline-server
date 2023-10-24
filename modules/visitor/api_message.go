package visitor

import (
	"errors"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

// 同步频道消息
func (v *Visitor) syncChannelMessage(c *wkhttp.Context) {
	var req config.SyncChannelMessageReq
	_, err := UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}

	visitorUid := req.LoginUID
	if visitorUid == "" {
		c.ResponseError(errors.New("login_uid is empty！"))
		return
	}
	vistor, err := v.db.QueryByUUID(visitorUid)
	if err != nil {
		v.Error("查询游客信息错误", zap.Error(err))
		c.ResponseErrorf("query visitor error！", err)
		return
	}
	if vistor == nil {
		c.ResponseWithStatus(400, "login_uid not found！")
		return
	}
	req.LoginUID = v.visitorUserUid
	v.message.SyncChannelMessage(c, &req)
}
