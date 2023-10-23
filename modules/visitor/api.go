// case 2: 游客初次访问，会自动创建一个visitor、一个group
package visitor

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/base/event"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/message"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/log"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkevent"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/mssola/useragent"
	"go.uber.org/zap"
)

type Visitor struct {
	ctx *config.Context
	log.Log
	db *DB
	// userDB  *user.DB
	groupDB *group.DB

	user    *user.User
	message *message.Message
	group   *group.Group

	visitorModel *Model
	// userModel    *user.Model
	groupModel *group.Model
	kefuModel  *user.Model

	restKefu    bool //是否重新分配客服
	lastLoginAt time.Time

	visitorUserUid  string //游客用户的通用uid
	visitorUserName string //游客用户的通用uid
}

func New(ctx *config.Context) *Visitor {
	v := &Visitor{
		ctx:          ctx,
		Log:          log.NewTLog("Visitor"),
		db:           NewDB(ctx),
		groupDB:      group.NewDB(ctx),
		user:         user.New(ctx),
		message:      message.New(ctx),
		group:        group.New(ctx),
		visitorModel: nil,
		// userModel:    nil,
		groupModel:      nil,
		kefuModel:       nil,
		restKefu:        false,
		lastLoginAt:     time.Now(),
		visitorUserUid:  "visitor",
		visitorUserName: "系统",
	}

	return v
}

func (v *Visitor) Route(r *wkhttp.WKHttp) {
	g := r.Group("/v1/visitor")
	{
		g.POST("/login", v.Login)
		g.POST("/message/sync", v.syncChannelMessage)
	}
}

type visitorLoginReq struct {
	Platform    string `json:"platform"`    //客户端平台
	Referer     string `json:"referer"`     //来源网站
	VisitorUuid string `json:"token"`       //游客uuid，可为空，当为空时，以新用户身份登录
	ProjectUuid string `json:"projectUuid"` //对接的项目uuid
	RestKefu    bool   `json:"resetKefu"`   //是否重新分配客服
}

func (r *visitorLoginReq) VisitorLoginCheck() error {
	if strings.TrimSpace(r.Platform) == "" {
		return errors.New("platform cannot be empty")
	}
	if strings.TrimSpace(r.Referer) == "" {
		return errors.New("referer cannot be empty")
	}
	if strings.TrimSpace(r.ProjectUuid) == "" {
		return errors.New("projectUuid cannot be empty")
	}
	return nil
}

type Resp struct {
	Data any `json:"data"`
}

type LoginInfo struct {
	// Uuid       string `json:"uuid"`
	Token      string `json:"token"`       //登录凭证，也就是visitor_uid
	UserUid    string `json:"user_uid"`    //visitor绑定的user_uid
	KefuUid    string `json:"kefu_uid"`    //visitor指定的kefu_uid
	ProjectUid string `json:"project_uid"` //visitor拜访的项目所在group_uid
	GroupUid   string `json:"channel_uid"` //visitor所在的group_uid
}

func (v *Visitor) Login(c *wkhttp.Context) {
	var req visitorLoginReq
	isEncrypt, err := UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}

	if err := req.VisitorLoginCheck(); err != nil {
		c.ResponseError(err)
		return
	}

	if err := v.initVisitorUser(); err != nil {
		v.Error("初始化visitor信息失败", zap.Error(err))
		c.ResponseError(err)
		return
	}

	v.restKefu = req.RestKefu
	// 获取referer
	referer := req.Referer
	// 是否为本地开发模式
	// isDev := strings.HasPrefix(req.Referer, "http://localhost")
	// 获取设备信息
	ua := useragent.New(c.Context.Request.UserAgent())
	// 如果是机器人
	if ua.Bot() {
		c.ResponseErrorWithStatus(errors.New("the current request is a bot"), 403)
		return
	}

	// 获取语言
	languages := strings.Join(util.RemoteLanguage(c.Context.Request), ",")

	// 获取ip
	ip := util.RemoteIp(c.Context.ClientIP())

	// 解析 ip 信息
	address := util.QueryAddress(ip)

	// 浏览器信息
	browser, _ := ua.Browser()

	// 名字
	name := "未知地區-網友"
	if address.Country != "" {
		name = address.Country
		if address.City != "" && name == address.City {
			name = fmt.Sprintf("%s-%s", name, address.City)
		}
		name = fmt.Sprintf("%s-網友", name)
	}

	// 查验项目uuid是否存在
	// 注意：这里的项目以群组的形式存在，直接查询是否存在该群组
	v.groupModel, err = v.groupDB.QueryWithGroupNo(req.ProjectUuid)
	if err != nil {
		v.Error("查询群信息错误", zap.Error(err))
		c.ResponseErrorf("query project error！", err)
		return
	}
	if v.groupModel == nil {
		//项目不存在
		c.ResponseErrorf("project not found", nil)
		return
	}
	projectUid := v.groupModel.GroupNo

	//查询该游客uuid是否存在
	visitorUid := strings.TrimSpace(req.VisitorUuid)
	// 默认为新游客
	isNewVisitor := true

	if len(visitorUid) != 0 {
		v.visitorModel, err = v.db.QueryByUUID(visitorUid)
		if err != nil {
			v.Error("查询游客信息错误", zap.Error(err))
			c.ResponseErrorf("query visitor error！", err)
			return
		}
		if v.visitorModel != nil {
			// 同时检查其绑定的group是否存在
			group, err := v.groupDB.QueryGroupWithGroupNo(v.visitorModel.GroupUid)
			if err != nil {
				v.Error("查询群信息错误", zap.Error(err))
				c.ResponseErrorf("query group error！", err)
				return
			}
			//如果存在，还要检查自己是否在群里面
			if group != nil {
				list, err := v.groupDB.QueryMembersWithGroupNo(group.GroupNo)
				if err != nil {
					v.Error("查询群成员信息错误", zap.Error(err))
					c.ResponseErrorf("query group error！", err)
					return
				}
				has := false
				for _, m := range list {
					if m.UID == v.visitorUserUid {
						has = true
						break
					}
				}
				//如果自己不在里面，表示该访客为新访客
				isNewVisitor = !has
			}
		}
	}

	if isNewVisitor {
		//如果是新用户，则开始创建用户信息
		visitor := &Model{
			Id:          0,
			Uuid:        util.GenerUUID(),
			UserUid:     v.visitorUserUid,
			KefuUid:     "",
			ProjectUid:  projectUid,
			GroupUid:    "",
			Comment:     "",
			Status:      0,
			Name:        name,
			Ip:          ip,
			Referer:     referer,
			Region:      address.Country,
			City:        address.City,
			Platform:    req.Platform,
			Device:      ua.Platform(),
			Os:          ua.OS(),
			Browser:     browser,
			Language:    languages,
			LastLoginAt: time.Now(),
			CreateAt:    time.Time{},
			UpdateAt:    time.Time{},
		}
		v.visitorModel = visitor
		err = v.registerHandler(c)
		if err != nil {
			c.ResponseErrorf("create visitor error！", err)
			return
		}
		go v.sendVisitorInfo()
	} else {
		//如果是老用户，则开始登录
		//记录上一次登录时间，防止被覆盖
		v.lastLoginAt = v.visitorModel.LastLoginAt

		//需要更新的字段
		v.visitorModel.Ip = ip
		v.visitorModel.Referer = referer
		v.visitorModel.Region = address.Country
		v.visitorModel.City = address.City
		v.visitorModel.Platform = req.Platform
		v.visitorModel.Device = ua.Platform()
		v.visitorModel.Os = ua.OS()
		v.visitorModel.Browser = browser
		v.visitorModel.Language = languages
		v.visitorModel.LastLoginAt = time.Now()
		err = v.loginHandler()
		if err != nil {
			c.ResponseErrorf("visitor login error！", err)
			return
		}
		go v.sendOnlineMsgToKefu()
	}
	//返回凭证，游客凭此凭证登录即可
	loginInfo := &LoginInfo{
		Token:      v.visitorModel.Uuid,
		UserUid:    v.visitorModel.UserUid,
		KefuUid:    v.visitorModel.KefuUid,
		ProjectUid: v.visitorModel.ProjectUid,
		GroupUid:   v.visitorModel.GroupUid,
	}
	if isEncrypt {
		encryptData := EncryptData(loginInfo)
		c.Writer.WriteString(encryptData)
	} else {
		c.Response(loginInfo)
	}
}

// 游客登录
// 将游客绑定的用户进行登录操作
func (v *Visitor) loginHandler() error {
	if v.visitorModel == nil {
		return errors.New("visitor does not exist")
	}
	// if v.userModel == nil {
	// 	return errors.New("user does not exist")
	// }
	// if v.userModel.Status == int(common.UserDisable) {
	// 	return errors.New("the user has been disabled")
	// }
	var err error
	kefuUid := v.visitorModel.KefuUid
	///查询客服信息
	v.kefuModel, err = v.user.Db().QueryByUID(kefuUid)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return err
	}
	kefu, err := v.assignKefu()
	if err != nil {
		return err
	}
	if kefu == nil {
		v.Error("游客未分配到客服")
		return errors.New("visitor not assigned to customer service")
	}
	hasOldKefu := false
	newKefuUid := kefu.UID
	//如果不存在群频道
	if v.visitorModel.GroupUid == "" {
		//则创建一个
		g, err := v.group.VisitorGroupCreate(&group.VisitorGroupCreateReq{
			CreateUid: v.visitorModel.UserUid,
			Name:      v.visitorModel.Name,
			Members: []string{
				kefu.UID,
			},
		})
		if err != nil {
			return err
		}
		v.visitorModel.GroupUid = g.GroupNo
		hasOldKefu = true
	} else {
		//当前分支：检查自己的专属客服是否已经被其它客服给更换了
		//先查找群成员
		members, err := v.groupDB.QueryMembersWithGroupNo(v.visitorModel.GroupUid)
		if err != nil {
			v.Error("查询群成员信息错误", zap.Error(err))
			return errors.New("查询群成员信息错误")
		}
		//检查群成员
		//如果一个人都没有
		n := len(members)
		//表示只有自己一个人，将客服拉进去
		if n == 1 {
			addMembers := []string{}
			addMembers = append(addMembers, newKefuUid)
			//则添加进去
			err = v.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
				OperatorUid:  v.visitorModel.UserUid,
				OperatorName: v.visitorUserName,
				Members:      addMembers,
				GroupNo:      v.visitorModel.GroupUid,
			})
			if err != nil {
				return err
			}
			//标识原来的客服还在，防止更换新的客服
			hasOldKefu = true
		} else {
			for _, m := range members {
				//检查是否存在当前的客服
				if m.UID == kefuUid {
					//表示原客服还在，还没有被更换
					hasOldKefu = true
					break
				}
			}
		}

		//如果原客服已经不在了，表示已经被换了客服
		//经过上面的处理，如果 hasOldKefu == false，表示现在群里至少有2个人了
		if !hasOldKefu {
			//找出群里的另一个客服，更新visitor上面的kefuUid
			for _, m := range members {
				//前提不是自己
				if m.UID != v.visitorModel.UserUid {
					//说明该客服已经被主动更换了，则使用主动更换的kefuUid
					newKefuUid = m.UID
					break
				}
			}
		}
	}

	//如果客服换了，则把原来的客服踢出群，然后拉入新客服入群
	if newKefuUid != kefuUid {
		v.visitorModel.KefuUid = newKefuUid
		//先添加新客服入群，如果添加成功，才能移除旧客服
		err = v.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
			OperatorUid:  v.visitorUserUid,
			OperatorName: v.visitorUserName,
			Members: []string{
				newKefuUid,
			},
			GroupNo: v.visitorModel.GroupUid,
		})
		if err != nil {
			return err
		}
		//移除旧客服
		if hasOldKefu {
			err = v.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
				OperatorUid:  v.visitorUserUid,
				OperatorName: v.visitorUserName,
				Members: []string{
					kefuUid,
				},
				GroupNo: v.visitorModel.GroupUid,
			})
			if err != nil {
				return err
			}
		}
	}
	//更新数据
	if err = v.db.Update(v.visitorModel); err != nil {
		v.Error("更新游客信息错误", zap.Error(err))
		return err
	}
	//不用换成token，凭uuid即可登录
	token := v.visitorModel.Uuid
	imResp, err := v.ctx.UpdateIMToken(config.UpdateIMTokenReq{
		UID:         v.visitorUserUid,
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

// 创建游客
func (v *Visitor) registerHandler(c *wkhttp.Context) error {
	kefu, err := v.assignKefu()
	if err != nil {
		v.Error("分配客服出现错误", zap.Error(err))
		return err
	}
	v.visitorModel.KefuUid = kefu.UID
	err = v.db.Insert(v.visitorModel)
	if err != nil {
		v.Error("新建游客信息错误", zap.Error(err))
		return err
	}

	//取出该visitor的uid
	visitorModel, err := v.db.QueryByUUID(v.visitorModel.Uuid)
	if err != nil {
		v.Error("查询游客信息错误", zap.Error(err))
		return err
	}
	if visitorModel == nil {
		v.Error("未找到刚注册的游客信息,uuid: " + v.visitorModel.Uuid)
		return errors.New("visitor does not exist")
	}

	//修改用户名
	v.visitorModel = visitorModel
	v.visitorModel.Name = fmt.Sprintf("%s-%d", v.visitorModel.Name, v.visitorModel.Id)

	//开启一个群聊频道，进行交流
	g, err := v.group.VisitorGroupCreate(&group.VisitorGroupCreateReq{
		CreateUid: v.visitorUserUid,
		Name:      v.visitorUserName,
		Members: []string{
			v.visitorModel.KefuUid,
		},
	})
	if err != nil {
		v.Error("创建游客群频道失败！", zap.Error(err))
		return err
	}

	v.visitorModel.GroupUid = g.GroupNo

	//更新
	if err = v.db.Update(v.visitorModel); err != nil {
		v.Error("创建群频道后，更新游客信息失败！", zap.Error(err))
		return err
	}

	return nil
}

/*
*
分配一个客服出来
如果有客服：客服不在线则重新分配，否则不分配 - 此方案已废弃，如果切换客服会带来一系列问题，不好处理
*
*/
func (v *Visitor) assignKefu() (*user.Model, error) {
	//如果已经存在了专属客服
	if v.kefuModel != nil {
		//如果客服可用
		if v.kefuModel.Status == 1 {
			//查看该客服是否在线
			onlineModel, err := v.user.OnlineService().QueryOnlineWithUID(v.kefuModel.UID)
			if err != nil {
				v.Error("查询在线信息错误", zap.Error(err))
				return nil, err
			}
			if onlineModel != nil {
				if onlineModel.Online == 1 {
					//如果专属客服在线，则返回原客服
					return v.kefuModel, nil
				} else {
					// 如果专属客服不在线，且离线超过10分钟
					t := time.Unix(int64(onlineModel.LastOffline), 0)
					//离线时间小于10分钟
					if time.Since(t).Minutes() < 10 {
						//不用换客服
						return v.kefuModel, nil
					}
					// 则根据 reset 决定是否分配新的客服
					if !v.restKefu {
						// 如果不重新分配，则返回原客服
						return v.kefuModel, nil
					}
					//否则继续执行，分配客服
				}
			}
		}
	}
	if v.groupModel == nil {
		return nil, errors.New("group does not exist")
	}
	groupNo := v.groupModel.GroupNo
	//从项目群组中分配一个客服出来作为该游客的专属客服
	//查找该群里有多少客服，除开超级管理员
	members, err := v.groupDB.QueryMembersWithGroupNo(groupNo)
	if err != nil {
		v.Error("查询群成员信息错误", zap.Error(err))
		return nil, err
	}
	n := len(members)
	if n == 0 {
		return nil, errors.New("invalid project")
	}
	//分配原则：
	//①不能分配给系统角色和创建者
	//②除非没有人了，才会考虑到分配给管理员
	//③如果没有可用客服，则返回错误
	var gmembers []*group.MemberModel
	for i := 0; i < n; i++ {
		member := members[i]
		if member.Role != 0 {
			continue
		}
		if member.Robot != 0 {
			continue
		}
		if member.Status != 1 {
			continue
		}
		if member.IsDeleted != 0 {
			continue
		}
		if member.UID == "fileHelper" {
			continue
		}
		if len(member.UID) < 16 {
			continue
		}
		gmembers = append(gmembers, member)
	}
	if len(gmembers) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}
	//再过滤一遍，排除掉系统账号
	var uids []string
	for _, gmember := range gmembers {
		uids = append(uids, gmember.UID)
	}
	userList, err := v.user.Db().QueryByUIDs(uids)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return nil, err
	}
	var users []*user.Model
	uids = []string{}
	for _, user := range userList {
		if user.Category == "system" {
			continue
		}
		uids = append(uids, user.UID)
		users = append(users, user)
	}
	//logic: 查询在线的用户
	onlines, err := v.user.OnlineService().QueryOnlineWithUIDs(uids)
	if err != nil {
		v.Error("查询在线用户错误", zap.Error(err))
		return nil, err
	}

	//如果没有在线客服, 但有专属客服，直接返回专属客服
	if len(onlines) == 0 && v.kefuModel != nil {
		return v.kefuModel, nil
	}

	if len(onlines) != 0 {
		//有在線人數，分配给在线的客服，筛选出在线客服uid
		uids = []string{}
		for _, online := range onlines {
			uids = append(uids, online.UID)
		}
	}
	//没有在线人数，则默认分配给所有离线客服
	// fmt.Printf("離線客服: %#v\n", uids)
	if len(uids) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}

	//确保每个客服在当日内分配到的游客数一样多：采用标记分配法
	// 	具体步骤如下：
	// 1.给每个人设置一个初始状态为未标记。这里给群成员备足标记📌为 xxxx.
	// 2.开始分配苹果时，从未标记的人中随机选择一个人进行分配，并将其标记为已分配过苹果。
	// 3.重复第2步，直到所有人都被标记为已分配过苹果。
	// 4.当所有人都被标记后，表示完成了一次轮回分配。清除所有人的标记，重复第2步开始新的轮回分配。

	//查找出未标记的群成员
	var noMarkMembers []*group.MemberModel
	var markMembers []*group.MemberModel
	for _, uid := range uids {
		for _, member := range members {
			if uid == member.UID {
				// 有标记
				if strings.HasSuffix(member.Remark, ".") {
					markMembers = append(markMembers, member)
				} else {
					noMarkMembers = append(noMarkMembers, member)
				}
			}
		}
	}

	//检查是否存在未标记的群成员
	if len(noMarkMembers) == 0 {
		if len(markMembers) == 0 {
			//特殊情况
			v.Error("member does not exist")
			return nil, errors.New("member does not exist")
		}
		//如果不存在未标记的群成员，则开启新的轮回分配
		//清除所有的标记
		for _, member := range markMembers {
			for strings.HasSuffix(member.Remark, ".") {
				member.Remark = strings.TrimSuffix(member.Remark, ".")
			}
			if err := v.groupDB.UpdateMemberRemark(member.Id, member.Remark); err != nil {
				v.Error("更新群成员备注失败", zap.Error(err))
				return nil, err
			}
		}
		noMarkMembers = markMembers
	}

	var addRemark = func(m *group.MemberModel) error {
		if strings.HasSuffix(m.Remark, ".") {
			return nil
		}
		m.Remark = fmt.Sprintf("%s.", m.Remark)
		if err := v.groupDB.UpdateMemberRemark(m.Id, m.Remark); err != nil {
			v.Error("更新群成员备注失败", zap.Error(err))
			return fmt.Errorf("更新群成员备注失败: %s", err)
		}
		return nil
	}

	//从未标记的群成员中随机挑选出一个
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomIndex := random.Intn(len(noMarkMembers))
	randomMember := noMarkMembers[randomIndex]
	uid := randomMember.UID
	for _, user := range users {
		if user.UID == uid {
			//如果已经在users中了，没必要再继续查询，直接返回即可
			if err := addRemark(randomMember); err != nil {
				return nil, err
			}
			return user, nil
		}
	}

	user, err := v.user.Db().QueryByUID(uid)
	if err != nil {
		v.Error("查询用户信息错误", zap.Error(err))
		return nil, err
	}
	if user == nil {
		v.Error("查询用户不存在")
		return nil, errors.New("system error! user does not exist")
	}
	v.kefuModel = user
	if err := addRemark(randomMember); err != nil {
		return nil, err
	}
	return user, nil
}

// 创建游客用户数据
func (v *Visitor) createUserModel() *user.Model {
	return &user.Model{
		AppID:             "",
		UID:               v.visitorUserUid,
		Name:              v.visitorUserName,
		Username:          "",
		Email:             "",
		Password:          util.GenerUUID(),
		Category:          "visitor",
		Sex:               0,
		ShortNo:           "39999",
		ShortStatus:       0,
		Zone:              "0086",
		Phone:             "13100000003",
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
		Vercode:           fmt.Sprintf("%s@%d", util.GenerUUID(), common.User),
		QRVercode:         fmt.Sprintf("%s@%d", util.GenerUUID(), common.QRCode),
		IsUploadAvatar:    0,
		Role:              "visitor",
		Robot:             0,
		MuteOfApp:         0,
		IsDestroy:         0,
		WXOpenid:          "",
		WXUnionid:         "",
		GiteeUID:          "",
		GithubUID:         "",
	}
}

// 处理注册游客和客服互为好友
// func (v *Visitor) addKefuFriend(uid string, kefuUid string) error {
// 	if uid == "" {
// 		v.Error("用户ID不能为空")
// 		return errors.New("用户ID不能为空")
// 	}
// 	isFriend, err := v.user.FriendDB().IsFriend(uid, kefuUid)
// 	if err != nil {
// 		v.Error("查询用户关系失败", zap.Error(err))
// 		return err
// 	}
// 	if isFriend {
// 		return nil
// 	}
// 	version := v.ctx.GenSeq(common.FriendSeqKey)
// 	err = v.user.FriendDB().Insert(&user.FriendModel{
// 		UID:     uid,
// 		ToUID:   kefuUid,
// 		Version: version,
// 	})
// 	if err != nil {
// 		v.Error("注册游客和客服互为好友失败", zap.Error(err))
// 		return err
// 	}
// 	return nil
// }

// 发送游客拜访信息到群频道
func (v *Visitor) sendVisitorInfo() {
	var sentContent string
	sentContent = fmt.Sprintf("来访游客: %s", v.visitorModel.Name)
	sentContent = fmt.Sprintf("%s\nip: %s", sentContent, v.visitorModel.Ip)
	sentContent = fmt.Sprintf("%s\n时间: %s", sentContent, time.Now().Local())
	sentContent = fmt.Sprintf("%s\n来源: %s", sentContent, v.visitorModel.Referer)
	sentContent = fmt.Sprintf("%s\n地区: %s", sentContent, v.visitorModel.Region)
	sentContent = fmt.Sprintf("%s\n城市: %s", sentContent, v.visitorModel.City)
	sentContent = fmt.Sprintf("%s\n平台: %s", sentContent, v.visitorModel.Platform)
	sentContent = fmt.Sprintf("%s\n设备: %s", sentContent, v.visitorModel.Device)
	sentContent = fmt.Sprintf("%s\n系统: %s", sentContent, v.visitorModel.Os)
	sentContent = fmt.Sprintf("%s\n浏览器: %s", sentContent, v.visitorModel.Browser)
	sentContent = fmt.Sprintf("%s\n语言: %s", sentContent, v.visitorModel.Language)

	//发送游客的来访信息给客服
	err := v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.visitorModel.UserUid,
		ChannelID:   v.visitorModel.GroupUid,
		ChannelType: common.ChannelTypeGroup.Uint8(),
		Payload: []byte(util.ToJson(map[string]interface{}{
			"content": sentContent,
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

// 发送上线信息到群频道，冷却时间30分钟
func (v *Visitor) sendOnlineMsgToKefu() {
	// 检查距离上一次登录时间是否已经达到30分钟
	if time.Since(v.lastLoginAt).Minutes() < 30 {
		return
	}
	//发送游客的上线信息给客服
	err := v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.visitorModel.UserUid,
		ChannelID:   v.visitorModel.GroupUid,
		ChannelType: common.ChannelTypeGroup.Uint8(),
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

func (v *Visitor) GenVisitorUserUID() string {
	return v.ctx.GetConfig().VisitorUIDPrefix + util.GenerUUID()
}

func (v *Visitor) initVisitorUser() error {
	userModel, err := v.user.Db().QueryByUID(v.visitorUserUid)
	if err != nil {
		v.Error("更新用戶信息错误", zap.Error(err))
		return err
	}
	if userModel != nil {
		// 已存在，无需创建，直接返回
		return nil
	}
	userModel = v.createUserModel()
	//插入数据库
	tx := v.user.Db().Tx()
	defer func() {
		if err := recover(); err != nil {
			v.Error(fmt.Sprintf("出现了恐慌: %s", err))
			tx.Rollback()
			panic(err)
		}
	}()
	// 事务处理
	var txEvent = func() error {
		if err := v.user.Db().InsertTx(userModel, tx); err != nil {
			v.Error("注册visitor用户失败", zap.Error(err))
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
		eventID, err := v.ctx.EventBegin(&wkevent.Data{
			Event: event.EventUserRegister,
			Type:  wkevent.Message,
			Data: map[string]interface{}{
				"uid": userModel.UID,
			},
		}, tx)
		if err != nil {
			v.Error("开启事件失败！", zap.Error(err))
			return err
		}

		//回调事件
		if err = tx.Commit(); err != nil {
			// tx.Rollback()
			// v.Error("数据库事务提交失败", zap.Error(err))
			return err
		}
		v.ctx.EventCommit(eventID)
		return nil
	}
	if err := txEvent(); err != nil {
		tx.Rollback()
		v.Error("数据库事务提交失败", zap.Error(err))
		return err
	}

	_, err = v.ctx.UpdateIMToken(config.UpdateIMTokenReq{
		UID:         userModel.UID,
		Token:       v.visitorModel.Uuid,
		DeviceFlag:  config.DeviceFlag(1), //注册设备的标记 0.手机 1.web 2.PC
		DeviceLevel: config.DeviceLevelSlave,
	})
	if err != nil {
		v.Error("更新IM的token失败！", zap.Error(err))
		return err
	}
	if v.ctx.GetConfig().ShortNo.NumOn {
		err = v.user.CommonService().SetShortnoUsed(userModel.ShortNo, "user")
		if err != nil {
			v.Error("设置短编号被使用失败！", zap.Error(err), zap.String("shortNo", userModel.ShortNo))
			return err
		}
	}
	return nil
}
