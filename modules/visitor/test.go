package visitor

import (
	"errors"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"go.uber.org/zap"
)

type TestLogic struct {
	*Visitor
}

func NewTestLogic(v *Visitor) *TestLogic {
	return &TestLogic{
		Visitor: v,
	}
}

func (v *Visitor) Test(c *wkhttp.Context) {
	NewTestLogic(v).Handler(c)
}

func (v *Visitor) Handler(c *wkhttp.Context) {
	name := c.Param("name")
	switch name {
	case "error":
		v.Error("错误日志测试", zap.Error(errors.New("错误内容xxxx")))
	}

	c.Response("ok")
}
