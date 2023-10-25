package visitor

import (
	"fmt"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"go.uber.org/zap"
)

// 游客登录
// 将游客绑定的用户进行登录操作
func (v *Visitor) loginHandler() error {
	//查询咨询群里的会员
	members, err := v.groupDB.QueryMembersWithGroupNo(v.channelID)
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
		err = v.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
			OperatorUid:  v.visitor.UID,
			OperatorName: "系统",
			Members: []string{
				v.kefu.UID,
			},
			GroupNo: v.channelID,
		})
		if err != nil {
			return err
		}
		if len(otherKefus) != 0 {
			err = v.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
				OperatorUid:  v.visitor.UID,
				OperatorName: "系统",
				Members:      otherKefus,
				GroupNo:      v.channelID,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *Visitor) redisLastLoginTimeKey() string {
	return fmt.Sprintf("visitorlastloginat:%s", v.channelID)
}

// 发送上线信息到群频道，冷却时间30分钟
func (v *Visitor) sendOnlineMsgToKefu() {
	key := v.redisLastLoginTimeKey()
	redisCli := v.ctx.Cache()
	value, err := redisCli.Get(key)
	if err != nil {
		v.Error("获取redis值错误", zap.Error(err))
	}
	if value != "" {
		//说明30分钟内登录过
		//返回，不执行
		return
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
	}
}
