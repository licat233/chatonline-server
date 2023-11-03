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
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/log"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/mssola/useragent"
	"go.uber.org/zap"
)

const CurrentChannelType = common.ChannelTypeGroup
const OperatorName = "system"

type Resp struct {
	Data any `json:"data"`
}

// 客服信息
type KefuInfo struct {
	UID    string `json:"uid"`    //客服的uid
	Name   string `json:"name"`   //客服的name
	Online bool   `json:"online"` //客服的在线状态
}

// 游客登录响应
type visitorLoginResp struct {
	IMToken   string   `json:"im_token"`    //游客登录im的token
	ChannelID string   `json:"channel_uid"` //聊天频道ID
	UserUid   string   `json:"user_uid"`    //游客绑定的user_uid
	KefuInfo  KefuInfo `json:"kefu_info"`   //客服信息
}

// 游客登录请求
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

	user    *user.User
	message *message.Message
	group   *group.Group

	currentProjectKefuGroup *group.Model

	req *visitorLoginReq

	info *Info //访客信息

	LoginInterval time.Duration //登录间隔时间，在此时间内，不会发送上线消息

	kefu    *user.Model
	visitor *user.Model

	channelID    string
	isOldVisitor bool
	KefuOnline   bool
}

func New(ctx *config.Context) *Visitor {
	v := &Visitor{
		ctx:                     ctx,
		Log:                     log.NewTLog("Visitor"),
		user:                    user.New(ctx),
		message:                 message.New(ctx),
		group:                   group.New(ctx),
		currentProjectKefuGroup: nil,
		req:                     nil,
		info:                    nil,
		LoginInterval:           time.Minute * 30,
		kefu:                    nil,
		visitor:                 nil,
		channelID:               "",
		isOldVisitor:            false,
		KefuOnline:              true,
	}

	return v
}

func (v *Visitor) Route(r *wkhttp.WKHttp) {
	g := r.Group("/v1/visitor")
	{
		g.POST("/login", v.Login)
		g.POST("/message/sync", v.syncChannelMessage)
		g.GET("/test/:name", v.Test)
	}
}

func (v *Visitor) Login(c *wkhttp.Context) {
	var req visitorLoginReq
	isEncrypt, err := tools.UnmarshalReqData(c, &req)
	if err != nil {
		c.ResponseError(err)
		return
	}

	if err := req.VisitorLoginCheck(); err != nil {
		c.ResponseError(err)
		return
	}

	v.req = &req

	platform := req.Platform

	// 获取referer
	referer := req.Referer

	// 获取设备信息
	ua := useragent.New(c.Context.Request.UserAgent())
	// 如果是机器人
	if ua.Bot() {
		c.ResponseErrorWithStatus(errors.New("abnormal requests"), 403)
		return
	}

	//如果当前不是debug模式
	if v.ctx.GetConfig().Mode != "debug" {
		//如果是本地链接，拒绝访问
		//如果是模拟器进行请求，拒绝访问
		//如果是PC平台，但显示却是手机平台，那就表示不正常的请求，直接拒绝响应
		if tools.IsLocalLink(referer) || ua.Mobile() == tools.IsDesktopDevice(req.Platform) {
			c.ResponseErrorWithStatus(errors.New("abnormal requests"), 403)
			return
		}
	}

	// 获取语言
	languages := strings.Join(tools.RemoteLanguage(c.Context.Request), ",")

	// 获取ip
	ip := tools.RemoteIp(c.Context.ClientIP())

	// 解析 ip 信息
	address := tools.QueryAddress(ip)

	// 浏览器信息
	browser, _ := ua.Browser()

	// 查验项目uuid是否存在
	// 注意：这里的项目以群组的形式存在，直接查询是否存在该群组
	v.currentProjectKefuGroup, err = v.group.GroupDB().QueryWithGroupNo(req.ProjectUid)
	if err != nil {
		v.Error("查询群信息错误", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if v.currentProjectKefuGroup == nil {
		//项目不存在
		c.ResponseErrorf("project not found", nil)
		return
	}

	v.info = &Info{
		Name:     fmt.Sprintf("%s的访客", v.currentProjectKefuGroup.Name),
		Ip:       ip,
		Referer:  referer,
		Region:   address.Country,
		City:     address.City,
		Platform: platform,
		Device:   ua.Platform(),
		Os:       ua.OS(),
		Browser:  browser,
		Language: languages,
	}

	//检查visitor是否存在
	if req.UserUid != "" {
		v.visitor, err = v.user.Db().QueryByUID(req.UserUid)
		if err != nil {
			v.Error("查询游客信息错误", zap.Error(err))
			tools.ServerError(c)
			return
		}
	}

	//如果visitor存在，则查看是否存在咨询群
	if v.visitor != nil {
		channelId := v.genChannelID()
		// 同时检查其绑定的group是否存在
		group, err := v.group.GroupDB().QueryGroupWithGroupNo(channelId)
		if err != nil {
			v.Error("查询群信息错误", zap.Error(err))
			tools.ServerError(c)
			return
		}
		//如果存在，还要检查自己是否在群里面
		if group != nil {
			exist, err := v.group.Service().ExistMember(group.GroupNo, v.visitor.UID)
			if err != nil {
				v.Error("查询是否在群内失败！", zap.Error(err))
				tools.ServerError(c)
				return
			}
			//如果自己在里面，则设为老游客
			v.isOldVisitor = exist
			if exist {
				//记录channel_uid
				v.channelID = group.GroupNo
				//顺便检查一下群名称是否正确
				if group.Name != v.genGroupName() {
					//修改一下群名称
					err = v.group.GroupDB().UpdateGroupName(group.GroupNo, v.genGroupName())
					if err != nil {
						v.Error("修改群昵称失败", zap.Error(err))
						tools.ServerError(c)
						return
					}
				}
			} else {
				//如果不在，表示该账号不能用于当前项目
				//因为wukongim无法单独只监听某个频道，所以只能一个项目使用一个visitor
				//一个页面存在多个visitor，请在缓存中设置键值对，projectUID-visitorUID
				//清空当前visitor信息
				v.visitor = nil
			}
		}

		//如果账号存在，当被当成了新游客，那会导致前端的老游客uid被覆盖，彻底丢失
	}

	//分配一个客服出来
	v.kefu, err = v.assignKefu()
	if err != nil {
		v.Error("分配客服出现错误", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if v.kefu == nil {
		v.Error("游客未分配到客服")
		tools.ServerError(c)
		return
	}

	onlineInfo, err := v.user.OnlineDB().QueryOnlineWithUID(v.kefu.UID)
	if err != nil {
		v.Error("查询kefu在线信息出错", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if onlineInfo == nil {
		//查找不到
		//默认在线
		v.KefuOnline = true
	} else {
		v.KefuOnline = onlineInfo.IsOnline()
	}

	if !v.isOldVisitor {
		err = v.registerHandler(c)
		if err != nil {
			v.Error("游客注册失败", zap.Error(err))
			tools.ServerError(c)
			return
		}
	} else {
		err = v.loginHandler()
		if err != nil {
			v.Error("游客登录失败", zap.Error(err))
			tools.ServerError(c)
			return
		}
	}

	token, err := v.GenVisitorIMToken()
	if err != nil {
		v.Error("获取token失败", zap.Error(err))
		tools.ServerError(c)
		return
	}

	//使用redis记录登录时间
	err = v.ctx.Cache().SetAndExpire(v.redisLastLoginTimeKey(), time.Now().String(), v.LoginInterval)
	if err != nil {
		v.Error("redis保存登录时间失败", zap.Error(err))
		tools.ServerError(c)
		return
	}

	//返回凭证，游客凭此凭证登录即可
	loginInfo := &visitorLoginResp{
		IMToken:   token,
		ChannelID: v.channelID,
		UserUid:   v.visitor.UID,
		KefuInfo: KefuInfo{
			UID:    v.kefu.UID,
			Name:   v.kefu.Name,
			Online: v.KefuOnline,
		},
	}
	if isEncrypt {
		encryptData := tools.EncryptData(loginInfo)
		c.Writer.Write(encryptData)
	} else {
		c.Response(loginInfo)
	}
}

func (v *Visitor) genChannelID() string {
	return tools.NewUUID(v.visitor.UID, v.req.ProjectUid)
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

func (v *Visitor) RemoveMembers(members []string) error {
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

func (v *Visitor) AddMembers(members []string) error {
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
func (v *Visitor) sendToVisitor(content string) error {
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
