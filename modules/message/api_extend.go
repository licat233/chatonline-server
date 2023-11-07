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

	limit := req.Limit
	pullMode := req.PullMode
	user_uid := req.LoginUID
	if user_uid == "" || limit <= 0 {
		return &syncChannelMessageResp{
			StartMessageSeq: req.EndMessageSeq,
			EndMessageSeq:   req.EndMessageSeq,
			PullMode:        pullMode,
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
				PullMode:        pullMode,
				Messages:        make([]*MsgSyncResp, 0),
			}, nil
		}
	}

	var lastMessageSeqMark uint32
	startMessageSeq := req.StartMessageSeq
	var baseMessages []*config.MessageResp
	var hasMore bool = true //1表示还有
	var loadHandler func() (*config.SyncChannelMessageResp, error)
	loadHandler = func() (*config.SyncChannelMessageResp, error) {
		resp, err := m.ctx.IMSyncChannelMessage(*req)
		if err != nil {
			return nil, err
		}
		n := len(resp.Messages)
		//没有limit那么多，表示已经取完了
		if n < limit {
			hasMore = false
		}
		//为0,表示已经取完了，且没有要处理的messages
		if n == 0 {
			hasMore = false
			return resp, nil
		}
		//如果是向下拉取的
		if pullMode == 0 {
			firstSeq := resp.Messages[0].MessageSeq
			//如果与上一次获取的相同，那就表示已经没有了，表示重复了，不用再处理，直接返回
			if firstSeq == lastMessageSeqMark {
				hasMore = false
				return resp, nil
			}
			//应该等于第一个消息的seq
			lastMessageSeqMark = firstSeq
		} else {
			lastSeq := resp.Messages[n-1].MessageSeq
			//如果与上一次获取的相同，那就表示已经没有了，表示重复了，不用再处理，直接返回
			if lastSeq == lastMessageSeqMark {
				hasMore = false
				return resp, nil
			}
			//应该等于最后一个的seq
			lastMessageSeqMark = lastSeq
		}

		//过滤掉一些系统通知，只要常规的消息
		var respMessages []*config.MessageResp
		for _, m := range resp.Messages {
			if m.FromUID != "" && m.GetContentType() < 100 {
				respMessages = append(respMessages, m)
			}
		}

		if pullMode == 0 {
			//如果是向下拉取，则应该在前面插入消息
			baseMessages = append(respMessages, baseMessages...)
		} else {
			//反之，在后面插入
			baseMessages = append(baseMessages, respMessages...)
		}

		m := len(baseMessages)
		//当前已经取得的消息数量不达标，并且还有更多，则再次获取
		if m < limit && hasMore {
			req.StartMessageSeq = lastMessageSeqMark
			//如果是向下拉取，应该-1，反之+1，表示不包含
			if pullMode == 0 {
				if lastMessageSeqMark == 0 {
					//已经是最后一个了，不能再请求了
					hasMore = false
					return resp, nil
				}
				req.StartMessageSeq = lastMessageSeqMark - 1
			} else {
				req.StartMessageSeq = lastMessageSeqMark + 1
			}
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
	//如果太多了，那就切割一下
	if len(baseMessages) > limit {
		//如果是向下取，则应该切割后面的
		if pullMode == 0 {
			start := len(baseMessages) - limit
			baseMessages = baseMessages[start:]
			//此时的start应该等于第一个的seq
			lastMessageSeqMark = baseMessages[0].MessageSeq
		} else {
			//切割前面的
			baseMessages = baseMessages[:limit]
			//此时的start应该等于最后一个的seq
			lastMessageSeqMark = baseMessages[len(baseMessages)-1].MessageSeq
		}
	}

	// //看是否存在end的限制，没有这种业务需求，暂时不用实现，增加不必要的复杂度
	// if req.EndMessageSeq != 0 {
	// 	if pullMode == 0 {
	// 		//不能小于end
	// 		if lastMessageSeqMark <= req.EndMessageSeq {

	// 		}
	// 	}
	// }

	if pullMode == 0 {
		lastMessageSeqMark--
	} else {
		lastMessageSeqMark++
	}

	resp.Messages = baseMessages
	resp.EndMessageSeq = lastMessageSeqMark //下一次请求，应该从这里开始
	resp.StartMessageSeq = startMessageSeq

	// fmt.Println("resp----messages-->", len(resp.Messages))
	data := newSyncChannelMessageResp(resp, user_uid, req.DeviceUUID, req.ChannelID, req.ChannelType, m.messageExtraDB, m.messageUserExtraDB, m.messageReactionDB, m.channelOffsetDB, m.deviceOffsetDB)
	if hasMore {
		data.More = 1
	} else {
		data.More = 0
	}

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
