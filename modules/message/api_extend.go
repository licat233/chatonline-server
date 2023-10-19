package message

import (
	"fmt"
	"net/http"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (co *Conversation) GetDeviceConversationMaxVersion(uid string, deviceUUID string) (int64, error) {
	return co.getDeviceConversationMaxVersion(uid, deviceUUID)
}

func (co *Conversation) GetUserConversationMaxVersion(uid string) (int64, error) {
	return co.getUserConversationMaxVersion(uid)
}

// 同步频道消息
func (m *Message) SyncChannelMessage(c *wkhttp.Context) {
	var req config.SyncChannelMessageReq
	if err := c.BindJSON(&req); err != nil {
		m.Error("数据格式有误！", zap.Error(err))
		c.ResponseError(errors.New("数据格式有误！"))
		return
	}

	user_uid := req.LoginUID

	// 如果当前用户不在群内，则直接返回空消息数组
	if req.ChannelType == common.ChannelTypeGroup.Uint8() {
		exist, err := m.groupService.ExistMember(req.ChannelID, user_uid)
		if err != nil {
			m.Error("查询是否在群内存在失败！", zap.Error(err))
			c.ResponseError(errors.New("查询是否在群内存在失败！"))
			return
		}
		if !exist {
			c.JSON(http.StatusOK, &syncChannelMessageResp{
				StartMessageSeq: req.EndMessageSeq,
				EndMessageSeq:   req.EndMessageSeq,
				PullMode:        req.PullMode,
				Messages:        make([]*MsgSyncResp, 0),
			})
			return
		}
	}

	req.LoginUID = c.GetLoginUID()
	resp, err := m.ctx.IMSyncChannelMessage(req)
	if err != nil {
		m.Error("同步频道内的消息失败！", zap.Error(err), zap.String("req", util.ToJson(req)))
		c.ResponseError(errors.New("同步频道内的消息失败！"))
		return
	}
	fmt.Println("resp----messages-->", len(resp.Messages))

	c.Response(newSyncChannelMessageResp(resp, c.GetLoginUID(), req.DeviceUUID, req.ChannelID, req.ChannelType, m.messageExtraDB, m.messageUserExtraDB, m.messageReactionDB, m.channelOffsetDB, m.deviceOffsetDB))
}

func (m *Message) SendMessage(channelID string, channelType uint8, fromUID string, payload map[string]interface{}) error {
	err := m.ctx.SendMessage(&config.MsgSendReq{
		Header: config.MsgHeader{
			RedDot: 1,
		},
		ChannelID:   channelID,
		ChannelType: channelType,
		FromUID:     fromUID,
		Payload:     []byte(util.ToJson(payload)),
	})
	if err != nil {
		m.Error("发送消息错误", zap.Error(err))
		return errors.New("发送消息错误")
	}
	return nil
}
