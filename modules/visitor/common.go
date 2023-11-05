package visitor

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
)

func (v *Visitor) getVisitorTokenCacheKey(channelID, visitorUID string) string {
	return fmt.Sprintf("%s:%s:%s", v.ctx.GetConfig().Cache.UIDTokenCachePrefix, channelID, visitorUID)
}

func (v *Visitor) redisLastLoginTimeCacheKey(channelID, visitorUID string) string {
	return fmt.Sprintf("visitorlastloginat:%s:%s", channelID, visitorUID)
}

func (v *Visitor) getVisitorUidCacheKey(token string) string {
	return fmt.Sprintf("visitorLoginedUid:%s", token)
}

func (v *Visitor) getVisitorUid(token string) (string, error) {
	key := v.getVisitorUidCacheKey(token)
	visitorUID, err := v.ctx.Cache().Get(key)
	if err != nil {
		v.Error("获取visitor的token错误", zap.Error(err))
		return "", errors.New("获取visitor的token错误")
	}
	return visitorUID, nil
}
