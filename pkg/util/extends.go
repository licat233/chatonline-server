package util

import (
	"log"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

// 地址信息
type AddressInfo struct {
	City      string  // 城市名称
	Provinces string  // 省份
	Country   string  // 国家名
	Continent string  // 洲名
	TimeZone  string  // 时区
	Latitude  float64 // 纬度
	Longitude float64 // 经度
}

// QueryAddress 查询ip地址信息
func QueryAddress(ipAddress string) *AddressInfo {
	dbfile := path.Join("configs/ipdb/GeoLite2-City.mmdb")
	db, err := geoip2.Open(dbfile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// If you are using strings that may be invalid, check that ip is not nil
	ip := net.ParseIP(ipAddress)
	record, err := db.City(ip)
	if err != nil {
		log.Fatal(err)
	}

	var info = new(AddressInfo)

	// 城市名称
	if len(record.City.Names) > 0 {
		info.City = record.City.Names["zh-CN"]
		//fmt.Println(record.City.Names["zh-CN"])
	}

	// 省份
	if len(record.Subdivisions) > 0 {
		if len(record.Subdivisions[0].Names) > 0 {
			info.Provinces = record.Subdivisions[0].Names["zh-CN"]
			//fmt.Println(record.Subdivisions[0].Names["zh-CN"])
		}
	}

	// 国家名
	if len(record.Country.Names) > 0 {
		info.Country = record.Country.Names["zh-CN"]
		//fmt.Println(record.Country.Names["zh-CN"])
	}

	// 洲名
	if len(record.Continent.Names) > 0 {
		info.Continent = record.Continent.Names["zh-CN"]
		//fmt.Println(record.Continent.Names["zh-CN"])
	}

	// 时区
	info.TimeZone = record.Location.TimeZone
	//fmt.Println(record.Location.TimeZone)

	// 纬度
	info.Latitude = record.Location.Latitude

	// 经度
	info.Longitude = record.Location.Longitude

	return info
}

// remoteIp 获取远程客户端的 IP
func RemoteIp(addr string) string {
	//可能会被仿冒，我们取最后一个IP，也就是原始IP
	strs := strings.Split(addr, ",")
	if ip := strings.TrimSpace(strs[len(strs)-1]); ip != "" {
		return strings.Split(ip, ":")[0]
	}

	return strings.Split(addr, ":")[0]
}

// remoteLanguage 获取客户端语言
func RemoteLanguage(r *http.Request) (languages []string) {
	acceptLanguage := r.Header.Get("Accept-Language")
	if acceptLanguage == "" {
		return
	}
	// 将Accept-Language字符串拆分成多个语言设置
	languageTags := strings.Split(acceptLanguage, ",")

	// 创建一个存储语言设置的切片
	languages = make([]string, 0, len(languageTags))

	// 迭代处理每个语言设置
	for _, tag := range languageTags {
		parts := strings.SplitN(tag, ";", 2)

		// 获取语言标签，忽略语言权重参数
		language := strings.TrimSpace(parts[0])

		// 添加到语言切片中
		languages = append(languages, language)
	}

	return
}
