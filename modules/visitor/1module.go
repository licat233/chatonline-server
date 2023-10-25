package visitor

import (
	_ "embed"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/register"
)

//go:embed swagger/api.yaml
var swaggerContent string

func init() {
	register.AddModule(func(ctx interface{}) register.Module {
		api := New(ctx.(*config.Context))
		return register.Module{
			Name: "visitor",
			SetupAPI: func() register.APIRouter {
				return api
			},
			Swagger: swaggerContent,
		}
	})
}
