package visitor

import (
	"errors"

	"strings"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

type SyncChannelMessageLogic struct {
	*Visitor
}

func NewSyncChannelMessageLogic(v *Visitor) *SyncChannelMessageLogic {
	return &SyncChannelMessageLogic{
		Visitor: v,
	}
}

func (v *Visitor) SyncChannelMessage(c *wkhttp.Context) {
	NewSyncChannelMessageLogic(v).Handler(c)
}

type syncChannelMessageReq struct {
	IMToken string `json:"im_token"` // 确保有权限拉取该频道的消息，必须已经登陆
	// UserUid         string          `json:"user_uid"`          // 用户UID，由Token查出
	ChannelID       string          `json:"channel_uid"`       // 聊天频道ID
	StartMessageSeq uint32          `json:"start_message_seq"` // 开始序列号
	EndMessageSeq   uint32          `json:"end_message_seq"`   // 结束序列号
	Limit           int             `json:"limit"`             // 每次同步数量限制
	PullMode        config.PullMode `json:"pull_mode"`         // 拉取模式
}

func (s *syncChannelMessageReq) ValueCheck() error {
	if s.ChannelID = strings.TrimSpace(s.ChannelID); s.ChannelID == "" {
		return errors.New("the requested data format is incorrect1")
	}
	if s.IMToken = strings.TrimSpace(s.IMToken); s.IMToken == "" {
		return errors.New("the requested data format is incorrect1")
	}
	return nil
}

// 同步客服频道消息
func (l *SyncChannelMessageLogic) Handler(c *wkhttp.Context) {
	var req syncChannelMessageReq
	isEncrypt, err := tools.UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}
	if err = req.ValueCheck(); err != nil {
		c.ResponseError(err)
		return
	}

	//校验token
	visitorUID, err := l.getVisitorUid(req.IMToken)
	if err != nil {
		c.ResponseWithStatus(500, err.Error())
		return
	}

	if visitorUID == "" {
		c.ResponseWithStatus(401, "authorization fail")
		return
	}
	in := &config.SyncChannelMessageReq{
		LoginUID:        visitorUID,
		DeviceUUID:      "",
		ChannelID:       req.ChannelID,
		ChannelType:     CurrentChannelType.Uint8(),
		StartMessageSeq: req.StartMessageSeq,
		EndMessageSeq:   req.EndMessageSeq,
		Limit:           req.Limit,
		PullMode:        req.PullMode,
	}
	msgList, err := l.message.SyncChannelMessage(in)
	if err != nil {
		l.Error("获取同频道消息失败", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if isEncrypt {
		encryptData := tools.EncryptData(msgList)
		c.Writer.Write(encryptData)
	} else {
		c.Response(msgList)
	}
}
