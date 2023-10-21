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
