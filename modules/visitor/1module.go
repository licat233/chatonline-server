package visitor

import (
	"embed"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/register"
)

//go:embed sql
var sqlFS embed.FS

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
			SQLDir:       register.NewSQLFS(sqlFS),
			Swagger:      swaggerContent,
			IMDatasource: register.IMDatasource{},
			// BussDataSource: register.BussDataSource{
			// 	ChannelGet: func(channelID string, channelType uint8, loginUID string) (*model.ChannelResp, error) {
			// 		if channelType != common.ChannelTypeGroup.Uint8() {
			// 			return nil, register.ErrDatasourceNotProcess
			// 		}
			// 		groupResp, err := api.user.UserService().GetUserDetail(channelID, loginUID)
			// 		if err != nil {
			// 			return nil, err
			// 		}
			// 		return user.NewChannelRespWithUserDetailResp(groupResp), nil
			// 	},
			// },
		}
	})
}
