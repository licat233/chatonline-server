package visitor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (v *Visitor) IpLimit(c *wkhttp.Context, maxCount int) bool {
	reqPath := c.Request.URL.Path
	ip := tools.RemoteIp(c.Context.ClientIP())
	key := fmt.Sprintf("ipVisits:%s:%s", reqPath, ip)
	redisCli := v.ctx.Cache()
	value, err := redisCli.Get(key)
	if err != nil {
		v.Error("获取ip访问次数错误", zap.Error(err))
		tools.ServerError(c)
		return false
	}
	if value == "" {
		value = "0"
	}

	count, _ := strconv.Atoi(value)
	count++
	err = redisCli.SetAndExpire(key, fmt.Sprint(count), time.Hour)
	if err != nil {
		v.Error("存储ip访问次数错误", zap.Error(err))
		tools.ServerError(c)
		return false
	}
	if count > maxCount {
		c.AbortWithStatusJSON(429, gin.H{
			"error": "Too many requests",
		})
		return false
	}
	return true
}

func (v *Visitor) IPRequestLimitMiddleware(maxCount int) wkhttp.HandlerFunc {
	return func(c *wkhttp.Context) {
		v.IpLimit(c, maxCount)
		c.Next()
	}
}

// func (v *Visitor) rejectMockDevice(c *wkhttp.Context) {

// }
