// case 2: 游客初次访问，会自动创建一个visitor、一个group
package visitor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
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
	if r.Platform = strings.TrimSpace(r.Platform); r.Platform == "" {
		return errors.New("the requested data format is incorrect1")
	}
	if r.Referer = strings.TrimSpace(r.Referer); r.Referer == "" {
		return errors.New("the requested data format is incorrect2")
	}
	if r.ProjectUid = strings.TrimSpace(r.ProjectUid); r.ProjectUid == "" {
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

// 由于这个系统的脑残设计，导致visitor里面的数据是所有请求共享的，所以不能用于纯数据
// 所以设计一个独立的结构体出来，每个请求单独创建一个
type LoginLogic struct {
	currentProjectKefuGroup *group.Model

	req *visitorLoginReq

	info *Info //访客信息

	LoginInterval time.Duration //登录间隔时间，在此时间内，不会发送上线消息

	kefu    *user.Model
	visitor *user.Model

	channelID    string
	isOldVisitor bool
	KefuOnline   bool

	*Visitor
}

func NewLoginLogic(v *Visitor) *LoginLogic {
	return &LoginLogic{
		currentProjectKefuGroup: nil,
		req:                     nil,
		info:                    nil,
		LoginInterval:           time.Minute * 30,
		kefu:                    nil,
		visitor:                 nil,
		channelID:               "",
		isOldVisitor:            false,
		KefuOnline:              true,
		Visitor:                 v,
	}
}

func (v *Visitor) Login(c *wkhttp.Context) {
	NewLoginLogic(v).Handler(c)
}

func (l *LoginLogic) Handler(c *wkhttp.Context) {
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

	l.req = &req

	platform := req.Platform

	// 获取referer
	referer := req.Referer

	// 获取设备信息
	ua := useragent.New(c.Context.Request.UserAgent())
	// 如果是机器人
	if ua.Bot() {
		c.ResponseWithStatus(403, "abnormal bot request")
		return
	}

	//如果当前不是debug模式
	if l.ctx.GetConfig().Mode != "debug" {
		//如果是本地链接，拒绝访问
		if tools.IsLocalLink(referer) {
			c.ResponseWithStatus(403, "abnormal device requests")
			return
		}
		//如果是模拟器进行请求，拒绝访问
		//如果是PC平台，但显示却是手机平台，那就表示不正常的请求，直接拒绝响应
		if tools.IsDesktopDevice(req.Platform) && ua.Mobile() {
			c.ResponseWithStatus(403, "abnormal device requests")
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
	l.currentProjectKefuGroup, err = l.group.GroupDB().QueryWithGroupNo(req.ProjectUid)
	if err != nil {
		l.Error("查询群信息错误", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if l.currentProjectKefuGroup == nil {
		//项目不存在
		c.ResponseErrorf("project not found", nil)
		return
	}

	l.info = &Info{
		Name:     fmt.Sprintf("%s的访客", l.currentProjectKefuGroup.Name),
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
		l.visitor, err = l.user.Db().QueryByUID(req.UserUid)
		if err != nil {
			l.Error("查询游客信息错误", zap.Error(err))
			tools.ServerError(c)
			return
		}
	}

	//如果visitor存在，则查看是否存在咨询群
	if l.visitor != nil {
		channelId := l.genChannelID()
		// 同时检查其绑定的group是否存在
		group, err := l.group.GroupDB().QueryGroupWithGroupNo(channelId)
		if err != nil {
			l.Error("查询群信息错误", zap.Error(err))
			tools.ServerError(c)
			return
		}
		//如果存在，还要检查自己是否在群里面
		if group != nil {
			exist, err := l.group.Service().ExistMember(group.GroupNo, l.visitor.UID)
			if err != nil {
				l.Error("查询是否在群内失败！", zap.Error(err))
				tools.ServerError(c)
				return
			}
			if exist {
				//如果自己在里面，则设为老游客
				l.isOldVisitor = true
				//记录channel_uid
				l.channelID = group.GroupNo
				//顺便检查一下群名称是否正确。不检查不修改，方便客服做备注
				// if group.Name != v.genGroupName() {
				// 	//修改一下群名称
				// 	err = v.group.GroupDB().UpdateGroupName(group.GroupNo, v.genGroupName())
				// 	if err != nil {
				// 		v.Error("修改群昵称失败", zap.Error(err))
				// 		tools.ServerError(c)
				// 		return
				// 	}
				// }
			} else {
				//如果不在，表示该账号不能用于当前项目
				//因为wukongim无法单独只监听某个频道，所以只能一个项目使用一个visitor
				//一个页面存在多个visitor，请在缓存中设置键值对，projectUID-visitorUID
				//清空当前visitor信息
				l.visitor = nil
			}
		}
		//如果账号存在，当被当成了新游客，那会导致前端的老游客uid被覆盖，彻底丢失
	}

	//分配一个客服出来
	l.kefu, err = l.assignKefu()
	if err != nil {
		l.Error("分配客服出现错误", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if l.kefu == nil {
		l.Error("游客未分配到客服")
		tools.ServerError(c)
		return
	}

	//获取客服在线状态
	onlines, err := l.user.OnlineDB().QueryOnlineWithUID(l.kefu.UID)
	if err != nil {
		l.Error("查询kefu在线信息出错", zap.Error(err))
		tools.ServerError(c)
		return
	}
	if len(onlines) == 0 {
		//查找不到
		//默认在线
		l.KefuOnline = true
	} else {
		for _, online := range onlines {
			if online.IsOnline() {
				l.KefuOnline = true
				break
			}
		}
	}

	if !l.isOldVisitor {
		//防止频繁注册，同一ip在1小时最多100次
		if !l.IpLimit(c, 100) {
			return
		}
		err = l.registerHandler(c)
		if err != nil {
			l.Error("游客注册失败", zap.Error(err))
			tools.ServerError(c)
			return
		}
	} else {
		err = l.loginHandler()
		if err != nil {
			l.Error("游客登录失败", zap.Error(err))
			tools.ServerError(c)
			return
		}
	}

	token, err := l.GenVisitorIMToken()
	if err != nil {
		l.Error("获取token失败", zap.Error(err))
		tools.ServerError(c)
		return
	}

	//使用redis记录登录时间
	err = l.ctx.Cache().SetAndExpire(l.redisLastLoginTimeCacheKey(l.channelID, l.visitor.UID), time.Now().String(), l.LoginInterval)
	if err != nil {
		l.Error("redis保存登录时间失败", zap.Error(err))
		tools.ServerError(c)
		return
	}

	//返回凭证，游客凭此凭证登录即可
	loginInfo := &visitorLoginResp{
		IMToken:   token,
		ChannelID: l.channelID,
		UserUid:   l.visitor.UID,
		KefuInfo: KefuInfo{
			UID:    l.kefu.UID,
			Name:   l.kefu.Name,
			Online: l.KefuOnline,
		},
	}
	if isEncrypt {
		encryptData := tools.EncryptData(loginInfo)
		c.Writer.Write(encryptData)
	} else {
		c.Response(loginInfo)
	}
}
