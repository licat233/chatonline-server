package visitor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/base/event"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkevent"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

// 创建游客
func (l *LoginLogic) registerHandler(c *wkhttp.Context) error {
	var err error
	l.visitor, err = l.createUserModel()
	if err != nil {
		l.Error("创建游客user数据出错:", zap.Error(err))
		return err
	}
	//插入数据库
	tx := l.user.Db().Tx()
	defer func() {
		if err := recover(); err != nil {
			l.Error(fmt.Sprintf("出现了恐慌: %s", err))
			tx.Rollback()
			panic(err)
		}
	}()

	var txEvent = func() error {
		if err = l.user.Db().InsertTx(l.visitor, tx); err != nil {
			l.Error("注册visitor用户失败", zap.Error(err))
			return err
		}

		// //构建设备信息
		// device := user.NewVisitorDeviceModel()
		// device.UID = userModel.UID
		// device.DeviceModel = v.visitorModel.Browser
		// device.DeviceID = v.visitorModel.Uuid
		// //注册设备信息
		// err = v.user.DeviceDB().InsertOrUpdateDeviceTx(device, tx)
		// if err != nil {
		// 	v.Error("添加用户设备信息失败", zap.Error(err))
		// 	return err
		// }

		// 添加客服好友, 游客身份，没必要添加客服好友
		// if err = v.addKefuFriend(userModel.UID, v.visitorModel.KefuUid); err != nil {
		// 	v.Error("添加客服好友出错", zap.Error(err))
		// 	return err
		// }

		//发送用户注册事件
		eventID, err := l.ctx.EventBegin(&wkevent.Data{
			Event: event.EventUserRegister,
			Type:  wkevent.Message,
			Data: map[string]interface{}{
				"uid": l.visitor.UID,
			},
		}, tx)
		if err != nil {
			l.Error("开启事件失败！", zap.Error(err))
			return err
		}

		//回调事件
		if err = tx.Commit(); err != nil {
			return err
		}
		l.ctx.EventCommit(eventID)
		return nil
	}

	if err := txEvent(); err != nil {
		tx.Rollback()
		l.Error("数据库事务提交失败", zap.Error(err))
		return err
	}

	l.visitor, err = l.user.Db().QueryByUID(l.visitor.UID)
	if err != nil {
		l.Error("查询user失败", zap.Error(err))
		return err
	}
	if l.visitor == nil {
		l.Error("查询不到刚注册的user", zap.Error(err))
		return errors.New("查询不到刚注册的user")
	}

	l.channelID = l.genChannelID()
	groupName := l.genGroupName()
	//开启一个群聊频道，进行交流
	g, err := l.group.VisitorGroupCreate(&group.VisitorGroupCreateReq{
		CreateUid:  l.visitor.UID,
		CreateName: OperatorName,
		GroupName:  groupName,
		GroupNo:    l.channelID,
		Notice:     l.genVisitorInfo(),
		Members:    []string{l.kefu.UID},
	})

	if err != nil {
		l.Error("创建游客群频道失败！", zap.Error(err))
		return err
	}

	//修改群头像
	err = l.group.UpdateVisitorGroupAvatar(tools.RandomAvatar(), g.GroupNo)
	if err != nil {
		l.Error("更新群头像失败", zap.Error(err))
		return err
	}

	l.info.Id = l.visitor.Id
	l.info.Name = groupName

	return nil
}

// 生成群名称，前提是visitor存在
func (l *LoginLogic) genGroupName() string {
	return fmt.Sprintf("%s%d", l.info.Name, l.visitor.Id)
}

// 创建游客用户数据
func (l *LoginLogic) createUserModel() (*user.Model, error) {
	var (
		shortNo = ""
		err     error
	)
	if l.ctx.GetConfig().ShortNo.NumOn {
		shortNo, err = l.user.CommonService().GetShortno()
		if err != nil {
			l.Error("获取短编号失败！", zap.Error(err))
			return nil, err
		}
	} else {
		shortNo = util.Ten2Hex(time.Now().UnixNano())
	}
	uid := l.ctx.GetConfig().VisitorUIDPrefix + genUuid()
	return &user.Model{
		AppID:             "",
		UID:               uid,
		Name:              l.info.Name,
		Username:          "",
		Email:             "",
		Password:          "",
		Category:          "visitor",
		Sex:               0,
		ShortNo:           shortNo,
		ShortStatus:       0,
		Zone:              "0086",
		Phone:             "13100000000",
		ChatPwd:           "",
		LockScreenPwd:     "",
		LockAfterMinute:   0,
		DeviceLock:        0,
		SearchByPhone:     0,
		SearchByShort:     0,
		NewMsgNotice:      0,
		MsgShowDetail:     0,
		VoiceOn:           0,
		ShockOn:           0,
		OfflineProtection: 0,
		Version:           0,
		Status:            1,
		Vercode:           "", // fmt.Sprintf("%s@%d", util.GenerUUID(), common.User),
		QRVercode:         "", // fmt.Sprintf("%s@%d", util.GenerUUID(), common.QRCode),
		IsUploadAvatar:    0,
		Role:              "visitor",
		Robot:             0,
		MuteOfApp:         0,
		IsDestroy:         0,
		WXOpenid:          "",
		WXUnionid:         "",
		GiteeUID:          "",
		GithubUID:         "",
	}, nil
}

// 发送游客拜访信息到群频道，不发也可以，因为已经在群公告显示了游客信息
func (l *LoginLogic) SendVisitorInfo() {
	var sentContent string = l.genVisitorInfo()

	//发送游客的来访信息给客服
	err := l.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     l.visitor.UID,
		ChannelID:   l.channelID,
		ChannelType: CurrentChannelType.Uint8(),
		Payload: []byte(util.ToJson(map[string]interface{}{
			"content": sentContent,
			"type":    common.Tip,
		})),
		Header: config.MsgHeader{
			RedDot: 0,
		},
	})
	if err != nil {
		l.Error("发送游客来访消息失败", zap.Error(err))
	}
}

func (l *LoginLogic) genVisitorInfo() string {
	var sentContent string
	sentContent = fmt.Sprintf("来访游客id: %d", l.info.Id)
	sentContent = fmt.Sprintf("%s\nip: %s", sentContent, l.info.Ip)
	sentContent = fmt.Sprintf("%s\n时间: %s", sentContent, time.Now().Format("2006-01-02 15:04:05"))
	sentContent = fmt.Sprintf("%s\n来源: %s", sentContent, l.info.Referer)
	sentContent = fmt.Sprintf("%s\n地区: %s", sentContent, l.info.Region)
	sentContent = fmt.Sprintf("%s\n城市: %s", sentContent, l.info.City)
	sentContent = fmt.Sprintf("%s\n平台: %s", sentContent, l.info.Platform)
	sentContent = fmt.Sprintf("%s\n设备: %s", sentContent, l.info.Device)
	sentContent = fmt.Sprintf("%s\n系统: %s", sentContent, l.info.Os)
	sentContent = fmt.Sprintf("%s\n浏览器: %s", sentContent, l.info.Browser)
	sentContent = fmt.Sprintf("%s\n语言: %s", sentContent, l.info.Language)
	return sentContent
}

func (v *LoginLogic) sendAutoReply() error {
	// 自动回复内容
	autoReplyContent := strings.TrimSpace(v.currentProjectKefuGroup.Notice)
	if len(autoReplyContent) == 0 {
		return nil
	}
	return v.sendToVisitor(autoReplyContent)
}
