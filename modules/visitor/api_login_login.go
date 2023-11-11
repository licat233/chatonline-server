package visitor

import (
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"go.uber.org/zap"
)

// 游客登录
// 将游客绑定的用户进行登录操作
func (l *LoginLogic) loginHandler() error {
	//查询咨询群里的会员
	members, err := l.group.GroupDB().QueryMembersWithGroupNo(l.channelID)
	if err != nil {
		l.Error("查询群会员错误", zap.Error(err))
		return err
	}
	var otherKefus []string
	var hasKefu bool
	for _, m := range members {
		if m.UID == l.kefu.UID {
			hasKefu = true
		} else if m.UID != l.visitor.UID {
			otherKefus = append(otherKefus, m.UID)
		}
	}

	//查看该客服是否在群里，如果不再则拉入，并把其它客服踢出群
	if !hasKefu {
		err = l.AddMembers([]string{
			l.kefu.UID,
		})
		if err != nil {
			return err
		}
		err = l.RemoveMembers(otherKefus)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *LoginLogic) loginedInShortTime() (bool, error) {
	key := l.redisLastLoginTimeCacheKey(l.channelID, l.visitor.UID)
	redisCli := l.ctx.Cache()
	value, err := redisCli.Get(key)
	if err != nil {
		l.Error("获取redis值错误", zap.Error(err))
		return false, err
	}
	return value != "", nil
}

// 发送上线信息到群频道，冷却时间30分钟
func (l *LoginLogic) sendOnlineMsgToKefu() error {
	yes, err := l.loginedInShortTime()
	if err != nil {
		return err
	}
	if yes {
		return nil
	}
	//发送游客的上线信息给客服
	err = l.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     l.visitor.UID,
		ChannelID:   l.channelID,
		ChannelType: CurrentChannelType.Uint8(),
		Payload: []byte(util.ToJson(map[string]interface{}{
			"content": "已上線",
			"type":    common.Tip,
		})),
		Header: config.MsgHeader{
			RedDot: 1,
		},
	})
	if err != nil {
		l.Error("发送游客来访消息失败", zap.Error(err))
		return err
	}
	return nil
}
