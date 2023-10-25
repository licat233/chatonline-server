package visitor_test

import (
	"fmt"
	"testing"

	"github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/visitor"
)

func TestDecryptJSONData(t *testing.T) {
	data := "rFipo5msnqOonVNwW4SZm4GiqpWdWGVZqp2emaiVo1hzWaCsrKRwX2CiqJqZpKCjqaRraWlnaGdaYFigo6WjnJusjamflFNwW52abJ5pb2dpbXJva2yZaWiRl2ZxnZpua5eXk5docm1rWmRWqp+cm6dZclqdbJtlaGhxbGlua2xqYGhmcphwmW9ra2Vma2qcnWlvZVhcU6ieqp2sg5mcpVNwramtnbU="
	res := visitor.DecryptJsonBase64Data([]byte(data))
	fmt.Println("结果:", string(res))
}

func TestLink(t *testing.T) {
	link1 := "http://example.com/somepage"
	link2 := "http://localhost:8080"
	link3 := "http://127.0.0.1:8000"
	link4 := "http://location.com"
	link5 := "/dsdasdw/dasdqw/dasd"
	link6 := "http://localhost/"
	link7 := "http://loc33alhost222/"

	fmt.Println(link1, "是本地连接:", visitor.IsLocalLink(link1))
	fmt.Println(link2, "是本地连接:", visitor.IsLocalLink(link2))
	fmt.Println(link3, "是本地连接:", visitor.IsLocalLink(link3))
	fmt.Println(link4, "是本地连接:", visitor.IsLocalLink(link4))
	fmt.Println(link5, "是本地连接:", visitor.IsLocalLink(link5))
	fmt.Println(link6, "是本地连接:", visitor.IsLocalLink(link6))
	fmt.Println(link7, "是本地连接:", visitor.IsLocalLink(link7))
}
