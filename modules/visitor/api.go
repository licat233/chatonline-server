// case 2: 游客初次访问，会自动创建一个visitor、一个group
package visitor

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/message"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/log"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/mssola/useragent"
	"go.uber.org/zap"
)

type Info struct {
	Id       int64
	Name     string
	Ip       string
	Referer  string
	Region   string
	City     string
	Platform string
	Device   string
	Os       string
	Browser  string
	Language string
}

type Visitor struct {
	ctx *config.Context
	log.Log
	// db *DB
	// userDB  *user.DB
	groupDB *group.DB

	user    *user.User
	message *message.Message
	group   *group.Group

	currentProjectGroupModel *group.Model

	lastLoginAt time.Time //[考虑删除]存储上一次登录时间，便于共享给其它函数使用

	visitorUserUid  string //游客用户的通用uid
	visitorUserName string //游客用户的通用uid

	req *visitorLoginReq

	info *Info //访客信息

	imToken string //游客登录im的token
}

type Resp struct {
	Data any `json:"data"`
}

type visitorLoginResp struct {
	IMToken   string `json:"im_token"`    //游客登录im的token
	ChannelID string `json:"channel_uid"` //聊天频道ID
	UserUid   string `json:"user_uid"`    //游客绑定的user_uid
}

type visitorLoginReq struct {
	Platform    string `json:"platform"`    //客户端平台
	Referer     string `json:"referer"`     //来源网站
	ProjectUuid string `json:"project_uid"` //对接的项目uid
	ChannelID   string `json:"channel_uid"` //聊天频道ID，可为空，当为空时，以新用户身份登录
	RestKefu    bool   `json:"reset_kefu"`  //是否重新分配客服, 默认false，true时当客服不在线时，将切换给其他客服
}

func (r *visitorLoginReq) VisitorLoginCheck() error {
	if strings.TrimSpace(r.Platform) == "" {
		return errors.New("platform cannot be empty")
	}
	if strings.TrimSpace(r.Referer) == "" {
		return errors.New("referer cannot be empty")
	}
	if strings.TrimSpace(r.ProjectUuid) == "" {
		return errors.New("project_uid cannot be empty")
	}
	return nil
}

func New(ctx *config.Context) *Visitor {
	v := &Visitor{
		ctx:                      ctx,
		Log:                      log.NewTLog("Visitor"),
		groupDB:                  group.NewDB(ctx),
		user:                     user.New(ctx),
		message:                  message.New(ctx),
		group:                    group.New(ctx),
		currentProjectGroupModel: nil,
		lastLoginAt:              time.Now(),
		visitorUserUid:           ctx.GetConfig().VisitorUIDPrefix + "visitor",
		visitorUserName:          "客服系统",
		req:                      nil,
		info:                     nil,
		imToken:                  "",
	}

	return v
}

func (v *Visitor) Route(r *wkhttp.WKHttp) {
	g := r.Group("/v1/visitor")
	{
		g.POST("/login", v.Login)
		g.POST("/message/sync", v.syncChannelMessage)
		g.GET("/test", v.Test)
	}
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

	v.req = &req

	// 获取referer
	referer := req.Referer
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

	//如果是PC平台，但显示却是手机平台，那就表示不正常的请求，直接拒绝响应
	if ua.Mobile() == IsDesktopDevice(req.Platform) {
		c.ResponseErrorWithStatus(errors.New("abnormal requests"), 400)
		return
	}

	// 名字
	name := "未知地區-訪客"
	if address.Country != "" {
		name = address.Country
		if address.City != "" && name != address.City {
			name = fmt.Sprintf("%s-%s", name, address.City)
		}
	}

	// 查验项目uuid是否存在
	// 注意：这里的项目以群组的形式存在，直接查询是否存在该群组
	v.currentProjectGroupModel, err = v.groupDB.QueryWithGroupNo(req.ProjectUuid)
	if err != nil {
		v.Error("查询群信息错误", zap.Error(err))
		c.ResponseErrorf("query project error！", err)
		return
	}
	if v.currentProjectGroupModel == nil {
		//项目不存在
		c.ResponseErrorf("project not found", nil)
		return
	}

	// 默认为新游客
	isNewVisitor := true

	if req.ChannelID != "" {
		// 同时检查其绑定的group是否存在
		group, err := v.groupDB.QueryGroupWithGroupNo(req.ChannelID)
		if err != nil {
			v.Error("查询群信息错误", zap.Error(err))
			c.ResponseErrorf("query group error！", err)
			return
		}
		//如果存在，还要检查自己是否在群里面
		if group != nil {
			exist, err := v.group.Service().ExistMember(req.ChannelID, v.visitorUserUid)
			if err != nil {
				v.Error("查询是否在群内存在失败！", zap.Error(err))
				c.ResponseError(errors.New("查询是否在群内存在失败！"))
				return
			}
			//如果自己不在里面，表示该访客为新访客
			isNewVisitor = !exist
		}
	}

	v.info = &Info{
		Name:     name,
		Ip:       ip,
		Referer:  referer,
		Region:   address.Country,
		City:     address.City,
		Platform: req.Platform,
		Device:   ua.Platform(),
		Os:       ua.OS(),
		Browser:  browser,
		Language: languages,
	}

	if isNewVisitor {
		err = v.registerHandler(c)
		if err != nil {
			c.ResponseErrorf("create visitor error！", err)
			return
		}
		go v.sendVisitorInfo()
	} else {
		err = v.loginHandler()
		if err != nil {
			c.ResponseErrorf("visitor login error！", err)
			return
		}
		go v.sendOnlineMsgToKefu()
	}
	//返回凭证，游客凭此凭证登录即可
	loginInfo := &visitorLoginResp{
		ChannelID: v.req.ChannelID,
		UserUid:   v.visitorUserUid,
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
	//分配一个客服出来
	kefu, err := v.assignKefu()
	if err != nil {
		return err
	}
	if kefu == nil {
		v.Error("游客未分配到客服")
		return errors.New("visitor not assigned to customer service")
	}
	//查询咨询群里的会员
	members, err := v.groupDB.QueryMembersWithGroupNo(v.req.ChannelID)
	if err != nil {
		v.Error("查询群会员错误", zap.Error(err))
		return err
	}
	var otherKefus []string
	var hasKefu bool
	for _, m := range members {
		if m.UID == kefu.UID {
			hasKefu = true
		} else {
			otherKefus = append(otherKefus, m.UID)
		}
	}

	//查看该客服是否在群里，如果不再则拉入，并把其它客服踢出群
	if !hasKefu {
		err = v.group.VisitorMemberAdd(&group.VisitorMemberAddReq{
			OperatorUid:  v.visitorUserUid,
			OperatorName: v.visitorUserName,
			Members: []string{
				kefu.UID,
			},
			GroupNo: v.req.ChannelID,
		})
		if err != nil {
			return err
		}
		if len(otherKefus) != 0 {
			err = v.group.VisitorMemberRemove(&group.VisitorMemberRemoveReq{
				OperatorUid:  v.visitorUserUid,
				OperatorName: v.visitorUserName,
				Members:      otherKefus,
				GroupNo:      v.req.ChannelID,
			})
			if err != nil {
				return err
			}
		}
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

	//开启一个群聊频道，进行交流
	g, err := v.group.VisitorGroupCreate(&group.VisitorGroupCreateReq{
		CreateUid: v.visitorUserUid,
		Name:      v.info.Name,
		Members: []string{
			kefu.UID,
		},
		GroupNo: v.req.ChannelID,
		Notice:  v.genVisitorInfo(),
	})

	if err != nil {
		v.Error("创建游客群频道失败！", zap.Error(err))
		return err
	}
	v.info.Id = g.Id

	if err = v.RegisterIMToken(); err != nil {
		return err
	}

	return nil
}

func (v *Visitor) RegisterIMToken() error {
	v.imToken = util.GenerUUID()
	imResp, err := v.ctx.UpdateIMToken(config.UpdateIMTokenReq{
		UID:         v.visitorUserUid,
		Token:       v.imToken,
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

/*
*
分配一个客服出来
如果有客服：客服不在线则重新分配，否则不分配 - 此方案已废弃，如果切换客服会带来一系列问题，不好处理
调用前，请检查好其绑定的group
*
*/
func (v *Visitor) assignKefu() (*user.Model, error) {
	//检查其绑定的group是否存在
	if v.currentProjectGroupModel == nil {
		return nil, errors.New("group does not exist")
	}

	//查找聊天群里所有的客服
	members, err := v.groupDB.QueryMembersWithGroupNo(v.req.ChannelID)
	if err != nil {
		v.Error("查询群成员错误", zap.Error(err))
		return nil, err
	}
	var uidList []string
	for _, gmember := range members {
		uidList = append(uidList, gmember.UID)
	}
	//查找在线的客服
	onlineUids, err := v.getOnlineUsers(uidList)
	if err != nil {
		return nil, err
	}
	if len(onlineUids) > 0 {
		//群里存在在线的客服，不用分配，随便返回一个即可
		kefuUid := onlineUids[0]
		kefu, err := v.user.Db().QueryByUID(kefuUid)
		if err != nil {
			v.Error("查询用户信息错误", zap.Error(err))
			return nil, err
		}
		return kefu, nil
	}
	//如果没有实时在线的客服，那就看是否有10分钟内离线的客服
	for _, uid := range uidList {
		exist, err := v.kefuIsOnline(uid)
		if err != nil {
			return nil, err
		}
		if exist {
			kefuUid := uid
			kefu, err := v.user.Db().QueryByUID(kefuUid)
			if err != nil {
				v.Error("查询用户信息错误", zap.Error(err))
				return nil, err
			}
			return kefu, nil
		}
	}

	//如果还是没有，那就从项目群里取一个在线的客服

	//从项目群组中分配一个客服出来作为该游客的专属客服
	//查找该群里有多少客服
	members, err = v.groupDB.QueryKefuMembers(v.currentProjectGroupModel.GroupNo)
	if err != nil {
		v.Error("查询群成员信息错误", zap.Error(err))
		return nil, err
	}
	if len(members) == 0 {
		//没有客服可用
		return nil, errors.New("the current project does not have customer service")
	}
	//再过滤一遍，排除掉系统账号
	var uids []string
	for _, gmember := range members {
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

	if len(onlines) != 0 {
		//有在線人數，分配给在线的客服，筛选出在线客服uid
		uids = []string{}
		for _, online := range onlines {
			uids = append(uids, online.UID)
		}
	}
	//没有在线人数，则默认分配给所有离线客服
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
	if err := addRemark(randomMember); err != nil {
		return nil, err
	}
	return user, nil
}

func (v *Visitor) genVisitorInfo() string {
	var sentContent string
	sentContent = fmt.Sprintf("来访游客id: %d", v.info.Id)
	sentContent = fmt.Sprintf("%s\nip: %s", sentContent, v.info.Ip)
	sentContent = fmt.Sprintf("%s\n时间: %s", sentContent, time.Now().Format("2006-01-02 15:04:05"))
	sentContent = fmt.Sprintf("%s\n来源: %s", sentContent, v.info.Referer)
	sentContent = fmt.Sprintf("%s\n地区: %s", sentContent, v.info.Region)
	sentContent = fmt.Sprintf("%s\n城市: %s", sentContent, v.info.City)
	sentContent = fmt.Sprintf("%s\n平台: %s", sentContent, v.info.Platform)
	sentContent = fmt.Sprintf("%s\n设备: %s", sentContent, v.info.Device)
	sentContent = fmt.Sprintf("%s\n系统: %s", sentContent, v.info.Os)
	sentContent = fmt.Sprintf("%s\n浏览器: %s", sentContent, v.info.Browser)
	sentContent = fmt.Sprintf("%s\n语言: %s", sentContent, v.info.Language)
	return sentContent
}

// 发送游客拜访信息到群频道
func (v *Visitor) sendVisitorInfo() {
	var sentContent string = v.genVisitorInfo()

	//发送游客的来访信息给客服
	err := v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.visitorUserUid,
		ChannelID:   v.req.ChannelID,
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
	// // 检查距离上一次登录时间是否已经达到30分钟
	// if time.Since(v.lastLoginAt).Minutes() < 30 {
	// 	return
	// }
	//发送游客的上线信息给客服
	err := v.ctx.SendMessage(&config.MsgSendReq{
		FromUID:     v.visitorUserUid,
		ChannelID:   v.req.ChannelID,
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

// func (v *Visitor) getOnlineMemberWithGroupNo(groupNo string) ([]string, error) {
// 	members, err := v.groupDB.QueryKefuMembers(groupNo)
// 	if err != nil {
// 		v.Error("查询群成员信息错误", zap.Error(err))
// 		return nil, err
// 	}
// 	n := len(members)
// 	if n == 0 {
// 		return nil, nil
// 	}
// 	var uids []string
// 	for _, gmember := range members {
// 		uids = append(uids, gmember.UID)
// 	}
// 	uids, err = v.getOnlineUsers(uids)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var res []string
// 	for _, uid := range uids {
// 		for _, gmember := range members {
// 			if uid == gmember.UID {
// 				res = append(res, uid)
// 			}
// 		}
// 	}
// 	return res, nil
// }

func (v *Visitor) getOnlineUsers(uids []string) ([]string, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	onlines, err := v.user.OnlineService().QueryOnlineWithUIDs(uids)
	if err != nil {
		v.Error("查询在线用户错误", zap.Error(err))
		return nil, err
	}
	res := []string{}
	for _, online := range onlines {
		res = append(res, online.UID)
	}
	return res, nil
}

func (v *Visitor) kefuIsOnline(uid string) (bool, error) {
	//离线小于10分钟，不算做离线
	onlineModel, err := v.user.OnlineService().QueryOnlineWithUID(uid)
	if err != nil {
		v.Error("查询在线信息错误", zap.Error(err))
		return false, err
	}
	if onlineModel == nil {
		return false, errors.New("kefu online info does not exist")
	}
	if onlineModel.Online == 1 {
		//如果专属客服在线，则返回原客服
		return true, nil
	} else {
		// 如果专属客服不在线，且离线超过10分钟
		t := time.Unix(int64(onlineModel.LastOffline), 0)
		//离线时间小于10分钟
		if time.Since(t).Minutes() < 10 {
			//不用换客服
			return true, nil
		}
		// 则根据 reset 决定是否分配新的客服
		if !v.req.RestKefu {
			// 如果不重新分配，则返回原客服
			return true, nil
		}
	}
	return false, nil
}
