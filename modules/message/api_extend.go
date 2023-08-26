package message

import "github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"

func (co *Conversation) GetDeviceConversationMaxVersion(uid string, deviceUUID string) (int64, error) {
	return co.getDeviceConversationMaxVersion(uid, deviceUUID)
}

func (co *Conversation) GetUserConversationMaxVersion(uid string) (int64, error) {
	return co.getUserConversationMaxVersion(uid)
}

// 同步频道消息
func (m *Message) SyncChannelMessage(c *wkhttp.Context) {
	m.syncChannelMessage(c)
}
