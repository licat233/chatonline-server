package visitor

import (
	"os"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"go.uber.org/zap"
)

// 游客登录
// 将游客绑定的用户进行登录操作
func (v *LoginLogic) loginHandler() error {
	//查询咨询群里的会员
	members, err := v.group.GroupDB().QueryMembersWithGroupNo(v.channelID)
	if err != nil {
		v.Error("查询群会员错误", zap.Error(err))
		return err
	}
	var otherKefus []string
	var hasKefu bool
	for _, m := range members {
		if m.UID == v.kefu.UID {
			hasKefu = true
		} else if m.UID != v.visitor.UID {
			otherKefus = append(otherKefus, m.UID)
		}
	}

	//查看该客服是否在群里，如果不再则拉入，并把其它客服踢出群
	if !hasKefu {
		err = v.AddMembers([]string{
			v.kefu.UID,
		})
		if err != nil {
			return err
		}
		err = v.RemoveMembers(otherKefus)
		if err != nil {
			return err
		}
	}
	//发送游客上线信息给客服
	go v.sendOnlineMsgToKefu()
	//发送客服不在线的信息给游客
	if !v.KefuOnline {
		content := os.Getenv("TS_OFFLINE_MESSAGE ")
		go v.sendKefuLeave(content)
	}
	return nil
}

func (v *LoginLogic) loginedInShortTime() (bool, error) {
	key := v.redisLastLoginTimeCacheKey(v.channelID, v.visitor.UID)
	redisCli := v.ctx.Cache()
	value, err := redisCli.Get(key)
	if err != nil {
		v.Error("获取redis值错误", zap.Error(err))
		return false, err
	}
	return value != "", nil
}

// 发送上线信息到群频道，冷却时间30分钟
func (v *LoginLogic) sendOnlineMsgToKefu() error {
	yes, err := v.loginedInShortTime()
	if err != nil {
		return err
	}
	if yes {
		return nil
	}
	// 2秒钟后发送
	time.Sleep(2 * time.Second)
	//发送游客的上线信息给客服
	err = v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.visitor.UID,
		ChannelID:   v.channelID,
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
		v.Error("发送游客来访消息失败", zap.Error(err))
		return err
	}
	return nil
}

// 发送客服离线消息给游客
func (v *LoginLogic) sendKefuLeave(content string) error {
	if v.KefuOnline {
		return nil
	}
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return nil
	}
	yes, err := v.loginedInShortTime()
	if err != nil {
		return err
	}
	if yes {
		return nil
	}
	//2秒钟后发送
	time.Sleep(2 * time.Second)
	return v.sendToVisitor(content)
}
