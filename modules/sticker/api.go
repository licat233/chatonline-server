package sticker

import (
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/log"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

type Sticker struct {
	ctx *config.Context
	log.Log
}

func New(ctx *config.Context) *Sticker {
	s := &Sticker{
		ctx: ctx,
		Log: log.NewTLog("Sticker"),
	}
	return s
}

// Route 路由配置
func (s *Sticker) Route(r *wkhttp.WKHttp) {
	// sticker := r.Group("/v1/sticker", s.ctx.AuthMiddleware(r))
	// {

	// }
	user := r.Group("/v1/sticker/user", s.ctx.AuthMiddleware(r))
	{
		user.GET("/category", s.getCategory)
	}

}
