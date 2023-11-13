package tools_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/pkg/util"
	"github.com/TangSengDaoDao/TangSengDaoDaoServer/tools"
	"github.com/mssola/useragent"
)

func TestScypto(t *testing.T) {
	text := "李光平"
	fmt.Println("[]rune: ", []rune(text))
	fmt.Println("明文->", text)
	// key := tools.GenSecretKey(nil, false)
	key := "233233"
	fmt.Println("key->", key)
	enstrng := tools.EncryptString(text, key)
	fmt.Println("密文->", string(enstrng))
	destring := tools.DecryptString(string(enstrng), key)
	fmt.Println("解密结果->", string(destring))
}

func TestRune(t *testing.T) {
	// 将rune类型转换为int32类型
	var r rune = '世'
	var i int = int(r)
	fmt.Printf("rune: %c, int: %d\n", r, i)

	// 将int32类型转换为字符串类型
	var s string = strconv.Itoa(i)
	fmt.Printf("string: %s\n", s)

	fmt.Println("反转")
	i, _ = strconv.Atoi(s)
	r = rune(i)
	fmt.Printf("rune: %c,byte: %b,  int: %d\n", r, byte(i), int(r))

	// s = "licat233"
	// fmt.Println("[]rune:", []rune(s))
	// fmt.Println("[]byte:", []byte(s))
}

func Test_ua(t *testing.T) {
	ua := useragent.New("ozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.43(0x18002b29) NetType/WIFI Language/zh_CN")
	fmt.Println(ua.Mobile())
	res := tools.IsDesktopDevice("iPhone")
	fmt.Println(res)
}

func TestMd5(t *testing.T) {
	fmt.Println(util.MD5(util.MD5("zxcvbnm,,,")))
}
