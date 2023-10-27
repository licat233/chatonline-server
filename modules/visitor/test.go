package visitor

import (
	"errors"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

func (v *Visitor) Test(c *wkhttp.Context) {
	name := c.Param("name")
	switch name {
	case "error":
		v.Error("错误日志测试", zap.Error(errors.New("错误内容xxxx")))
	}

	c.Response("ok")
}
