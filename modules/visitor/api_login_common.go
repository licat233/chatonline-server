package visitor

import (
	"errors"
	"strings"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func genUuid() string {
	u := uuid.New()

	// 移除分隔符
	uuidWithoutHyphen := strings.ReplaceAll(u.String(), "-", "")
	return uuidWithoutHyphen
}

func (l *LoginLogic) genChannelID() string {
	return tools.NewUUID(l.visitor.UID, l.req.ProjectUid)
}

func (l *LoginLogic) registerIMToken(token string) (err error) {
	imResp, err := l.ctx.UpdateIMToken(config.UpdateIMTokenReq{
		UID:         l.visitor.UID,
		Token:       token,
		DeviceFlag:  config.DeviceFlag(config.Web), //注册设备的标记 0.app 1.web 2.PC
		DeviceLevel: config.DeviceLevelSlave,
	})
	if err != nil {
		l.Error("更新IM的token失败！", zap.Error(err))
		return err
	}
	if imResp.Status == config.UpdateTokenStatusBan {
		return errors.New("this account has been banned")
	}
	return nil
}

func (l *LoginLogic) GenVisitorIMToken() (string, error) {
	//获取缓存的token
	key := l.getVisitorTokenCacheKey(l.channelID, l.visitor.UID)
	token, err := l.ctx.Cache().Get(key)
	if err != nil {
		l.Error("获取visitor的token错误", zap.Error(err))
		return "", errors.New("获取visitor的token错误")
	}
	if token != "" {
		return token, nil
	}
	token = util.GenerUUID()
	err = l.registerIMToken(token)
	if err != nil {
		return "", errors.New("注册im token失败！")
	}
	err = l.ctx.Cache().SetAndExpire(key, token, l.ctx.GetConfig().Cache.TokenExpire)
	if err != nil {
		l.Error("设置token缓存失败！", zap.Error(err))
		return "", errors.New("设置token缓存失败！")
	}
	//还需要设置token所对应的用户uid，便于后期通过token查询其user_uid
	err = l.ctx.Cache().SetAndExpire(l.getVisitorUidCacheKey(token), l.visitor.UID, l.ctx.GetConfig().Cache.TokenExpire)
	if err != nil {
		l.Error("设置token缓存失败！", zap.Error(err))
		return "", errors.New("设置token缓存失败！")
	}
	return token, nil
}

func (l *LoginLogic) RemoveMembers(members []string) error {
	if len(members) == 0 {
		return nil
	}
	//排除自己
	uids := []string{}
	for _, uid := range members {
		if uid != l.visitor.UID {
			uids = append(uids, uid)
		}
	}
	err := l.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
		OperatorUid:  l.visitor.UID,
		OperatorName: OperatorName,
		Members:      uids,
		GroupNo:      l.channelID,
	})
	if err != nil {
		l.Error("移除群会员失败", zap.Error(err))
		return err
	}
	return nil
}

func (l *LoginLogic) AddMembers(members []string) error {
	if len(members) == 0 {
		return nil
	}
	//排除自己
	uids := []string{}
	for _, uid := range members {
		if uid != l.visitor.UID {
			uids = append(uids, uid)
		}
	}
	err := l.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
		OperatorUid:  l.visitor.UID,
		OperatorName: OperatorName,
		Members:      uids,
		GroupNo:      l.channelID,
	})
	if err != nil {
		l.Error("添加群会员失败", zap.Error(err))
		return err
	}
	return nil
}

// 发送消息给游客
func (l *LoginLogic) sendToVisitor(content string) error {
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return nil
	}
	contentType := common.Text
	if tools.IsImageURL(content) {
		contentType = common.Image
	}
	err := l.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     l.kefu.UID,
		ChannelID:   l.channelID,
		ChannelType: CurrentChannelType.Uint8(),
		Payload: []byte(util.ToJson(map[string]interface{}{
			"content": content,
			"type":    contentType,
		})),
		Header: config.MsgHeader{
			RedDot: 1,
		},
	})
	if err != nil {
		l.Error("发送消息给游客失败", zap.Error(err))
		return err
	}
	return nil
}

// 发送客服离线消息给游客
func (l *LoginLogic) sendKefuLeave(content string) error {
	if l.KefuOnline {
		return nil
	}
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return nil
	}
	yes, err := l.loginedInShortTime()
	if err != nil {
		return err
	}
	if yes {
		return nil
	}
	return l.sendToVisitor(content)
}
