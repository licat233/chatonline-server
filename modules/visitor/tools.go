package visitor

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/wkhttp"
)

// 解析被加密的request数据，req必须为指针
func UnmarshalReqData(c *wkhttp.Context, req any) (isEncrypt bool, err error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false, errors.New("read request body faild")
	}
	if len(body) == 0 {
		return false, errors.New("request body is nil")
	}
	jsonStr := string(body)
	err = json.Unmarshal([]byte(jsonStr), req)
	if err != nil {
		//可能不是json
		jsonStr = DecryptJSONData(body)
		err = json.Unmarshal([]byte(jsonStr), req)
		if err != nil {
			return true, errors.New("the requested data format is incorrect")
		}
	}

	return true, nil
}

// 加密数据
func EncryptData(data interface{}) string {
	input, _ := json.Marshal(data)
	return EncryptString(string(input), "")
}

// 解密json数据
func DecryptJSONData(data []byte) string {
	return DecryptJSONString(string(data), "")
}

// 加密字符串
func EncryptString(input string, secretKey string) string {
	if secretKey == "" {
		secretKey = GenSecretKey(nil, false, false)
	}
	n := len(secretKey)
	encrypted := ""
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		iv := int(secretKey[i%n])
		encryptedCharCode := charCode + iv
		encrypted += string(rune(encryptedCharCode))
	}
	return encrypted
}

// 解密字符串
func DecryptString(input string, secretKey string) string {
	if secretKey == "" {
		date := time.Now()
		secretKey = GenSecretKey(&date, false, false)
	}
	n := len(secretKey)
	decrypted := ""
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		iv := int(secretKey[i%n])
		decryptedCharCode := charCode - iv
		decrypted += string(rune(decryptedCharCode))
	}
	return decrypted
}

// 解密json字符串
func DecryptJSONString(input string, secretKey string) string {
	date := time.Now()
	if secretKey == "" {
		secretKey = GenSecretKey(&date, false, false)
	}
	n := len(secretKey)
	decrypted := ""
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		iv := int(secretKey[i%n])
		decryptedCharCode := charCode - iv
		decrypted += string(rune(decryptedCharCode))
	}

	if !IsJSON(decrypted) {
		decrypted = DecryptString(input, GenSecretKey(&date, false, true))
		if !IsJSON(decrypted) {
			return ""
		}
	}
	return decrypted
}

func GetPreviousMinuteDate(date *time.Time) time.Time {
	if date == nil {
		now := time.Now()
		date = &now
	}
	return date.Add(time.Minute * -1)
}

func GenSecretKey(date *time.Time, reverse bool, previousMinute bool) string {
	if date == nil {
		now := time.Now()
		date = &now
	}
	if previousMinute {
		*date = GetPreviousMinuteDate(date)
	}
	template := "2006-01-02-15-04"
	if reverse {
		template = "04-15-02-01-2006"
	}
	return date.Format(template)
}

// 解析被压缩的request数据，req必须为指针
func UnmarshalReqZipData(c *wkhttp.Context, req any) (compressType string, err error) {
	if req == nil {
		panic("req is nil")
	}
	// 读取请求体数据
	compressedData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.ResponseErrorWithStatus(errors.New("failed to read request body"), http.StatusInternalServerError)
		return
	}
	var decompressedData []byte
	compressType = c.Request.Header.Get("Content-Encoding")
	switch compressType {
	case "deflate":
		decompressedData, err = decompressedFlateData(compressedData)
	case "gzip":
		decompressedData, err = decompressGzipData(compressedData)
	case "zlib":
		decompressedData, err = decompressZlibData(compressedData)
	default:
		if err = c.BindJSON(req); err != nil {
			c.ResponseError(errors.New("the requested data format is incorrect"))
		}
		return "", err
	}
	if err != nil {
		c.ResponseError(errors.New("the requested data format is incorrect"))
		return
	}
	err = json.Unmarshal(decompressedData, req)
	if err != nil {
		c.ResponseError(errors.New("failed to json unmarshal request data"))
		return
	}
	return
}

func decompressedFlateData(compressedData []byte) ([]byte, error) {
	// 创建deflate解压缩器
	reader := flate.NewReader(bytes.NewReader(compressedData))
	defer reader.Close()

	return io.ReadAll(reader)
}

func compressFlateData(data any) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	defer writer.Close()

	_, err = writer.Write(jsonData)
	if err != nil {
		return nil, err
	}

	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompressGzipData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func compressGzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	defer writer.Close()

	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Flush()
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompressZlibData(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	zlibReader, err := zlib.NewReader(reader)
	defer zlibReader.Close()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(zlibReader)
}

func compressZlibData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zlibWriter := zlib.NewWriter(&buf)
	defer zlibWriter.Close()

	_, err := zlibWriter.Write(data)
	if err != nil {
		return nil, err
	}

	err = zlibWriter.Flush()
	if err != nil {
		return nil, err
	}

	err = zlibWriter.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}
