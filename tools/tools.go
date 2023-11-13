package tools

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
	"github.com/oschwald/geoip2-golang"
)

func IsDesktopDevice(platform string) bool {
	// 将 platform 转换为小写，以便进行比较
	platform = strings.ToLower(platform)

	// 判断是否为电脑设备
	if strings.Contains(platform, "win") || strings.Contains(platform, "mac") ||
		(strings.Contains(platform, "linux") && !strings.Contains(platform, "arm")) {
		return true
	}

	return false
}

func FindEqualStrings(arr1 []string, arr2 []string) []string {
	// 创建一个空的字符串切片，用于存储相同的字符串
	equalStrings := []string{}

	// 创建一个 map，用于存储 arr1 中的字符串
	stringMap := make(map[string]bool)

	// 将 arr1 中的字符串添加到 map 中
	for _, str := range arr1 {
		stringMap[str] = true
	}

	// 遍历 arr2，如果字符串在 map 中存在，则添加到 commonStrings 中
	for _, str := range arr2 {
		if stringMap[str] {
			equalStrings = append(equalStrings, str)
		}
	}

	return equalStrings
}

func IsLocalLink(link string) bool {
	parsedURL, err := url.Parse(link)
	if err != nil {
		// fmt.Println("无法解析链接:", err)
		//乱七八糟的字符串，肯定不是一个合格的链接，当作本地链接处理
		return true
	}

	host := strings.ToLower(parsedURL.Host)

	// 检查主机部分是否为 localhost、127.0.0.1 或空
	if host == "localhost" || host == "127.0.0.1" || host == "" {
		return true
	}

	// 检查链接是否以 http://localhost 或 http://127.0.0.1 开头
	if strings.HasPrefix(link, "http://localhost") || strings.HasPrefix(link, "http://127.0.0.1") {
		return true
	}

	return false
}

func NewUUID(uuids ...string) string {
	return MD5(strings.Join(uuids, ""))
}

func ServerError(c *wkhttp.Context) {
	c.ResponseWithStatus(500, "server error")
	c.Abort()
}

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

// 判断某个字符串是不是图片的url
func IsImageURL(input string) bool {
	// 将URL字符串解析为URL对象
	u, err := url.Parse(input)
	if err != nil {
		return false
	}

	// 获取URL路径的文件名部分
	filename := u.Path[strings.LastIndex(u.Path, "/")+1:]

	// 检查文件名的扩展名是否为图片格式
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif"}
	for _, ext := range imageExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}

	return false
}

// 移除重复的元素
func SliceRemoveDuplicates(strSlice []string) []string {
	encountered := map[string]bool{} // 用于记录出现过的字符串
	result := []string{}             // 存储去重后的结果

	for _, str := range strSlice {
		if !encountered[str] {
			encountered[str] = true
			result = append(result, str)
		}
	}

	return result
}

func RandomAvatar() string {
	var store []string
	for i := 0; i <= 487; i++ {
		if i <= 100 {
			store = append(store, fmt.Sprintf("%d.jpg", i))
		}
		store = append(store, fmt.Sprintf("avatar%d.jpeg", i))
	}
	newRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomIndex := newRand.Intn(len(store))
	avatarName := store[randomIndex]
	storeUrl := os.Getenv("TS_AVATAR_FILE_PATH")
	if storeUrl == "" {
		storeUrl = "https://licat-store.oss-cn-hongkong.aliyuncs.com/lib/comment/avatar"
	}
	return fmt.Sprintf("%s/%s", storeUrl, avatarName)
}

func CleanURL(urlString string) string {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return ""
	}

	// 获取域名和路径
	cleanURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	return cleanURL
}

func IsExceedsLengthLimit(str string, limit int) bool {
	return utf8.RuneCountInString(str) > limit
}

func GenerateString(chars []string, sep string, limit int) string {
	var result string
	for _, char := range chars {
		if IsExceedsLengthLimit(result+char, limit) {
			return result
		}
		result += char
	}
	return result
}
