package visitor

import (
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/group"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/message"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/user"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/log"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

type Visitor struct {
	ctx *config.Context
	log.Log

	user    *user.User
	message *message.Message
	group   *group.Group
}

func New(ctx *config.Context) *Visitor {
	v := &Visitor{
		ctx:     ctx,
		Log:     log.NewTLog("Visitor"),
		user:    user.New(ctx),
		message: message.New(ctx),
		group:   group.New(ctx),
	}

	return v
}

func (v *Visitor) Route(r *wkhttp.WKHttp) {
	g := r.Group("/v1/visitor")
	{
		g.POST("/login", v.Login)
		g.POST("/message/sync", v.SyncChannelMessage)
		g.GET("/test/:name", v.Test)
	}
}
