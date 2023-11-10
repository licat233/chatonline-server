package visitor

import (
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
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
	c.Response("ok")
}
