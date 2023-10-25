package visitor

import (
	"errors"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

type syncChannelMessageReq struct {
	Token           string          `json:"token"`             //验证token
	ProjectUid      string          `json:"project_uid"`       // 项目所在群的group_no
	StartMessageSeq uint32          `json:"start_message_seq"` // 开始序列号
	EndMessageSeq   uint32          `json:"end_message_seq"`   // 结束序列号
	Limit           int             `json:"limit"`             // 每次同步数量限制
	PullMode        config.PullMode `json:"pull_mode"`         // 拉取模式
}

// 同步频道消息
func (v *Visitor) syncChannelMessage(c *wkhttp.Context) {
	var req syncChannelMessageReq
	_, err := UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}
	if req.Token == "" {
		c.ResponseErrorWithStatus(errors.New("authorization fail"), 401)
		return
	}

	in := &config.SyncChannelMessageReq{
		LoginUID:        v.visitorUserUid,
		DeviceUUID:      "",
		ChannelID:       v.req.ChannelID,
		ChannelType:     common.ChannelTypeGroup.Uint8(),
		StartMessageSeq: req.StartMessageSeq,
		EndMessageSeq:   req.EndMessageSeq,
		Limit:           req.Limit,
		PullMode:        req.PullMode,
	}

	v.message.SyncChannelMessage(c, in)
}
