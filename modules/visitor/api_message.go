package visitor

import (
	"errors"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

type syncChannelMessageReq struct {
	IMToken         string          `json:"im_token"`          // 确保有权限拉取该频道的消息
	ChannelID       string          `json:"channel_uid"`       // 聊天频道ID
	StartMessageSeq uint32          `json:"start_message_seq"` // 开始序列号
	EndMessageSeq   uint32          `json:"end_message_seq"`   // 结束序列号
	Limit           int             `json:"limit"`             // 每次同步数量限制
	PullMode        config.PullMode `json:"pull_mode"`         // 拉取模式
}

func (s *syncChannelMessageReq) ValueCheck() error {
	if s.ChannelID == "" {
		return errors.New("the requested data format is incorrect1")
	}
	if s.IMToken == "" {
		return errors.New("the requested data format is incorrect1")
	}
	return nil
}

// 同步客服频道消息
func (v *Visitor) syncChannelMessage(c *wkhttp.Context) {
	var req syncChannelMessageReq
	_, err := UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}
	if err = req.ValueCheck(); err != nil {
		c.ResponseErrorWithStatus(err, 400)
		return
	}

	//校验token
	token, err := v.GenVisitorIMToken()
	if err != nil {
		c.ResponseErrorWithStatus(err, 500)
		return
	}

	if token != req.IMToken {
		c.ResponseErrorWithStatus(errors.New("authorization fail"), 401)
		return
	}

	in := &config.SyncChannelMessageReq{
		LoginUID:        v.visitor.UID,
		DeviceUUID:      "",
		ChannelID:       req.ChannelID,
		ChannelType:     CurrentChannelType.Uint8(),
		StartMessageSeq: req.StartMessageSeq,
		EndMessageSeq:   req.EndMessageSeq,
		Limit:           req.Limit,
		PullMode:        req.PullMode,
	}

	v.message.SyncChannelMessage(c, in)
}
