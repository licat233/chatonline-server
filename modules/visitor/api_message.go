package visitor

import "github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"

// 同步频道消息
func (v *Visitor) syncChannelMessage(c *wkhttp.Context) {
	v.message.SyncChannelMessage(c)
}
