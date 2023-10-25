// case 2: 游客初次访问，会自动创建一个visitor、一个group
package visitor

import (
	"errors"
	"fmt"
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
	groupDB *group.DB

	user    *user.User
	message *message.Message
	group   *group.Group

	currentProjectGroupModel *group.Model

	req *visitorLoginReq

	info *Info //访客信息

	LoginInterval time.Duration //登录间隔时间，在此时间内，不会发送上线消息

	kefu    *user.Model
	visitor *user.Model

	channelID string
}

const CurrentChannelType = common.ChannelTypeGroup
const OperatorName = "system"

type Resp struct {
	Data any `json:"data"`
}

type visitorLoginResp struct {
	IMToken   string `json:"im_token"`    //游客登录im的token
	ChannelID string `json:"channel_uid"` //聊天频道ID
	UserUid   string `json:"user_uid"`    //游客绑定的user_uid
}

type visitorLoginReq struct {
	Platform   string `json:"platform"`    //客户端平台
	Referer    string `json:"referer"`     //来源网站
	ProjectUid string `json:"project_uid"` //对接的项目uid
	UserUid    string `json:"user_uid"`    //游客所使用的user_uid
	RestKefu   bool   `json:"reset_kefu"`  //是否重新分配客服, 默认false，true时当客服不在线时，将切换给其他客服
}

func (r *visitorLoginReq) VisitorLoginCheck() error {
	if strings.TrimSpace(r.Platform) == "" {
		return errors.New("the requested data format is incorrect1")
	}
	if strings.TrimSpace(r.Referer) == "" {
		return errors.New("the requested data format is incorrect2")
	}
	if strings.TrimSpace(r.ProjectUid) == "" {
		return errors.New("the requested data format is incorrect3")
	}
	r.UserUid = strings.TrimSpace(r.UserUid)
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
		req:                      nil,
		info:                     nil,
		LoginInterval:            time.Minute * 30,
		kefu:                     nil,
		visitor:                  nil,
		channelID:                "",
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
	//判断是本地开发时，发送过来的，且当前不是调试模式
	if IsLocalLink(referer) && v.ctx.GetConfig().Mode != "debug" {
		//则不允许登录
		c.ResponseErrorWithStatus(errors.New("the requested data format is incorrect"), 403)
		return
	}
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

	// 查验项目uuid是否存在
	// 注意：这里的项目以群组的形式存在，直接查询是否存在该群组
	v.currentProjectGroupModel, err = v.groupDB.QueryWithGroupNo(req.ProjectUid)
	if err != nil {
		v.Error("查询群信息错误", zap.Error(err))
		ServerError(c)
		return
	}
	if v.currentProjectGroupModel == nil {
		//项目不存在
		c.ResponseErrorf("project not found", nil)
		return
	}

	v.info = &Info{
		Name:     fmt.Sprintf("%s的访客", v.currentProjectGroupModel.Name),
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

	// 默认为新游客
	isNewVisitor := true

	//检查visitor是否存在
	if req.UserUid != "" {
		v.visitor, err = v.user.Db().QueryByUID(req.UserUid)
		if err != nil {
			v.Error("查询游客信息错误", zap.Error(err))
			ServerError(c)
			return
		}
	}

	//如果不是新用户，则查看自己的群是否存在
	if v.visitor != nil {
		channelId := v.genChannelID()
		// 同时检查其绑定的group是否存在
		group, err := v.groupDB.QueryGroupWithGroupNo(channelId)
		if err != nil {
			v.Error("查询群信息错误", zap.Error(err))
			ServerError(c)
			return
		}
		//如果存在，还要检查自己是否在群里面
		if group != nil {
			exist, err := v.group.Service().ExistMember(v.channelID, v.visitor.UID)
			if err != nil {
				v.Error("查询是否在群内失败！", zap.Error(err))
				ServerError(c)
				return
			}
			//如果自己在里面，则设为老游客
			if exist {
				isNewVisitor = false
				//否则记录channel_uid
				v.channelID = channelId
				//顺便检查一下群名称是否正确
				if group.Name != v.genGroupName() {
					//修改一下群名称
					err = v.groupDB.UpdateGroupName(v.channelID, v.genGroupName())
					if err != nil {
						v.Error("修改群昵称失败", zap.Error(err))
						ServerError(c)
						return
					}
				}
			}
		}
	}

	//分配一个客服出来
	v.kefu, err = v.assignKefu()
	if err != nil {
		v.Error("分配客服出现错误", zap.Error(err))
		ServerError(c)
		return
	}
	if v.kefu == nil {
		v.Error("游客未分配到客服")
		ServerError(c)
		return
	}

	if isNewVisitor {
		err = v.registerHandler(c)
		if err != nil {
			v.Error("游客注册失败", zap.Error(err))
			ServerError(c)
			return
		}
		// go v.sendVisitorInfo() //不用发了，公告有显示
	} else {
		err = v.loginHandler()
		if err != nil {
			v.Error("游客登录失败", zap.Error(err))
			ServerError(c)
			return
		}
		v.sendOnlineMsgToKefu()
	}

	token, err := v.GenVisitorIMToken()
	if err != nil {
		v.Error("获取token失败", zap.Error(err))
		ServerError(c)
		return
	}

	//使用redis记录登录时间
	err = v.ctx.Cache().SetAndExpire(v.redisLastLoginTimeKey(), time.Now().String(), v.LoginInterval)
	if err != nil {
		v.Error("redis保存登录时间失败", zap.Error(err))
		ServerError(c)
		return
	}

	//返回凭证，游客凭此凭证登录即可
	loginInfo := &visitorLoginResp{
		IMToken:   token,
		ChannelID: v.channelID,
		UserUid:   v.visitor.UID,
	}
	if isEncrypt {
		encryptData := EncryptData(loginInfo)
		c.Writer.WriteString(encryptData)
	} else {
		c.Response(loginInfo)
	}
}

func (v *Visitor) genChannelID() string {
	return NewUUID(v.visitor.UID, v.req.ProjectUid)
}

func (v *Visitor) registerIMToken(token string) (err error) {
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

func (v *Visitor) GenVisitorIMToken() (string, error) {
	flag := config.Web
	//获取缓存的token
	key := fmt.Sprintf("%s%d%s", v.ctx.GetConfig().Cache.UIDTokenCachePrefix, flag, v.visitor.UID)
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

	return token, nil
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
