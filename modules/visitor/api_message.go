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
	if err := c.BindJSON(&req); err != nil {
		v.Error("数据格式有误！", zap.Error(err))
		c.ResponseError(errors.New("数据格式有误！"))
		return
	}
	visitorUid := req.LoginUID
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
	req.LoginUID = vistor.UserUid
	v.message.SyncChannelMessage(c, &req)
}
