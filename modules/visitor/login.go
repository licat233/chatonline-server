package visitor

import (
	"fmt"
	"strings"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"go.uber.org/zap"
)

// 游客登录
// 将游客绑定的用户进行登录操作
func (v *Visitor) loginHandler() error {
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
	//发送客服不在线的信息给客服
	if !v.KefuOnline {
		go v.sendKefuLeave("感謝您光臨！很抱歉，我們的客服目前不在線上。如果您有任何問題或需要幫助，請在下方留言，我們會儘快回覆您的訊息。感謝您的耐心等待。")
	}
	return nil
}

func (v *Visitor) redisLastLoginTimeKey() string {
	return fmt.Sprintf("visitorlastloginat:%s", v.channelID)
}

func (v *Visitor) loginedInShortTime() (bool, error) {
	key := v.redisLastLoginTimeKey()
	redisCli := v.ctx.Cache()
	value, err := redisCli.Get(key)
	if err != nil {
		v.Error("获取redis值错误", zap.Error(err))
		return false, err
	}
	return value != "", nil
}

// 发送上线信息到群频道，冷却时间30分钟
func (v *Visitor) sendOnlineMsgToKefu() error {
	yes, err := v.loginedInShortTime()
	if err != nil {
		return err
	}
	if yes {
		return nil
	}
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
func (v *Visitor) sendKefuLeave(content string) error {
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
	return v.sendToVisitor(content)
}
