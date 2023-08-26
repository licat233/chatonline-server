package visitor

import (
	"errors"
	"strings"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

func (v *Visitor) SyncAllMessages(c *wkhttp.Context) {
	var req struct {
		Token string `json:"token"` // 其实就是游客的uuid
	}
	if err := c.BindJSON(&req); err != nil {
		v.Error("数据格式有误！", zap.Error(err))
		c.ResponseError(errors.New("数据格式有误！"))
		return
	}
	uuid := req.Token
	if uuid = strings.TrimSpace(uuid); uuid == "" {
		c.ResponseError(errors.New("token cannot be empty"))
		return
	}

	visitorModel, err := v.db.QueryByUUID(req.Token)
	if err != nil {
		v.Error("查询游客信息错误", zap.Error(err))
		c.ResponseErrorf("query visitor error！", err)
		return
	}
	if visitorModel == nil {
		c.ResponseErrorf("visitor not exsit！", err)
		return
	}
	//获取该游客的所有好友，即客服
}
