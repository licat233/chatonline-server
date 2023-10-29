package message

import (
	"fmt"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
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
func (m *Message) SyncChannelMessage(req *config.SyncChannelMessageReq) (respData *syncChannelMessageResp, e error) {
	defer func() {
		if fail := recover(); fail != nil {
			m.Error("出现系统错误", zap.Error(errors.New(fmt.Sprint(fail))))
			e = errors.New("server error")
			// c.ResponseErrorWithStatus(errors.New("server error"), 500)
		}
	}()

	user_uid := req.LoginUID
	if user_uid == "" {
		return &syncChannelMessageResp{
			StartMessageSeq: req.EndMessageSeq,
			EndMessageSeq:   req.EndMessageSeq,
			PullMode:        req.PullMode,
			Messages:        make([]*MsgSyncResp, 0),
		}, nil
	}

	// 如果当前用户不在群内，则直接返回空消息数组
	if req.ChannelType == common.ChannelTypeGroup.Uint8() {
		exist, err := m.groupService.ExistMember(req.ChannelID, user_uid)
		if err != nil {
			m.Error("查询是否在群内存在失败！", zap.Error(err))
			return nil, err
		}
		if !exist {
			return &syncChannelMessageResp{
				StartMessageSeq: req.EndMessageSeq,
				EndMessageSeq:   req.EndMessageSeq,
				PullMode:        req.PullMode,
				Messages:        make([]*MsgSyncResp, 0),
			}, nil
		}
	}

	var endMessageSeq uint32
	startMessageSeq := req.StartMessageSeq
	var baseMessages []*config.MessageResp
	var hasMore int = 1 //1表示还有
	var loadHandler func() (*config.SyncChannelMessageResp, error)
	loadHandler = func() (*config.SyncChannelMessageResp, error) {
		resp, err := m.ctx.IMSyncChannelMessage(*req)
		if err != nil {
			return nil, err
		}
		n := len(resp.Messages)
		if n < req.Limit {
			hasMore = 0
		}
		if n == 0 {
			hasMore = 0
			return resp, nil
		}
		if endMessageSeq == resp.Messages[n-1].MessageSeq {
			hasMore = 0
			return resp, nil
		}
		//过滤掉一些系统通知，只要常规的消息
		for _, m := range resp.Messages {
			if m.FromUID != "" && m.GetContentType() < 100 {
				baseMessages = append(baseMessages, m)
			}
		}
		endMessageSeq = resp.Messages[n-1].MessageSeq
		m := len(baseMessages)
		//数量不达标，则再次获取
		if m == 0 || m < req.Limit {
			req.StartMessageSeq = endMessageSeq
			resp, err = loadHandler()
			if err != nil {
				return nil, err
			}
		}
		return resp, nil
	}

	resp, err := loadHandler()
	if err != nil {
		m.Error("同步频道内的消息失败！", zap.Error(err), zap.String("req", util.ToJson(req)))
		return nil, err
	}
	resp.Messages = baseMessages
	resp.EndMessageSeq = endMessageSeq
	resp.StartMessageSeq = startMessageSeq

	// fmt.Println("resp----messages-->", len(resp.Messages))
	data := newSyncChannelMessageResp(resp, user_uid, req.DeviceUUID, req.ChannelID, req.ChannelType, m.messageExtraDB, m.messageUserExtraDB, m.messageReactionDB, m.channelOffsetDB, m.deviceOffsetDB)
	data.More = hasMore
	return data, nil
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
