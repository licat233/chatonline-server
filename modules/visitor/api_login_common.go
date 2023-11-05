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

func (v *LoginLogic) genChannelID() string {
	return tools.NewUUID(v.visitor.UID, v.req.ProjectUid)
}

func (v *LoginLogic) registerIMToken(token string) (err error) {
	imResp, err := v.ctx.UpdateIMToken(config.UpdateIMTokenReq{
		UID:         v.visitor.UID,
		Token:       token,
		DeviceFlag:  config.DeviceFlag(config.Web), //注册设备的标记 0.app 1.web 2.PC
		DeviceLevel: config.DeviceLevelSlave,
	})
	if err != nil {
		v.Error("更新IM的token失败！", zap.Error(err))
		return err
	}
	if imResp.Status == config.UpdateTokenStatusBan {
		return errors.New("this account has been banned")
	}
	return nil
}

func (v *LoginLogic) GenVisitorIMToken() (string, error) {
	//获取缓存的token
	key := v.getVisitorTokenCacheKey(v.channelID, v.visitor.UID)
	token, err := v.ctx.Cache().Get(key)
	if err != nil {
		v.Error("获取visitor的token错误", zap.Error(err))
		return "", errors.New("获取visitor的token错误")
	}
	if token != "" {
		return token, nil
	}
	token = util.GenerUUID()
	err = v.registerIMToken(token)
	if err != nil {
		return "", errors.New("注册im token失败！")
	}
	err = v.ctx.Cache().SetAndExpire(key, token, v.ctx.GetConfig().Cache.TokenExpire)
	if err != nil {
		v.Error("设置token缓存失败！", zap.Error(err))
		return "", errors.New("设置token缓存失败！")
	}
	//还需要设置token所对应的用户uid，便于后期通过token查询其user_uid
	err = v.ctx.Cache().SetAndExpire(v.getVisitorUidCacheKey(token), v.visitor.UID, v.ctx.GetConfig().Cache.TokenExpire)
	if err != nil {
		v.Error("设置token缓存失败！", zap.Error(err))
		return "", errors.New("设置token缓存失败！")
	}
	return token, nil
}

func (v *LoginLogic) RemoveMembers(members []string) error {
	if len(members) == 0 {
		return nil
	}
	//排除自己
	uids := []string{}
	for _, uid := range members {
		if uid != v.visitor.UID {
			uids = append(uids, uid)
		}
	}
	err := v.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
		OperatorUid:  v.visitor.UID,
		OperatorName: OperatorName,
		Members:      uids,
		GroupNo:      v.channelID,
	})
	if err != nil {
		v.Error("移除群会员失败", zap.Error(err))
		return err
	}
	return nil
}

func (v *LoginLogic) AddMembers(members []string) error {
	if len(members) == 0 {
		return nil
	}
	//排除自己
	uids := []string{}
	for _, uid := range members {
		if uid != v.visitor.UID {
			uids = append(uids, uid)
		}
	}
	err := v.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
		OperatorUid:  v.visitor.UID,
		OperatorName: OperatorName,
		Members:      uids,
		GroupNo:      v.channelID,
	})
	if err != nil {
		v.Error("添加群会员失败", zap.Error(err))
		return err
	}
	return nil
}

// 发送消息给游客
func (v *LoginLogic) sendToVisitor(content string) error {
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return nil
	}
	contentType := common.Text
	if tools.IsImageURL(content) {
		contentType = common.Image
	}
	err := v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.kefu.UID,
		ChannelID:   v.channelID,
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
		v.Error("发送消息给游客失败", zap.Error(err))
		return err
	}
	return nil
}
