package visitor

import (
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

func (v *Visitor) Test(c *wkhttp.Context) {
	c.Response("ok")
}
