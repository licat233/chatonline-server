package visitor

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	if IsJSON(body) {
		err = json.Unmarshal(body, req)
		if err != nil {
			return false, errors.New("the requested data format is incorrect")
		}
		return false, nil
	}
	// 开始解码
	decodedJSONData := DecryptJsonBase64Data(body)
	if len(decodedJSONData) == 0 {
		//不是json数据
		return false, errors.New("the requested data format is incorrect")
	}
	err = json.Unmarshal(decodedJSONData, req)
	if err != nil {
		//无法映射到req中
		return false, errors.New("the requested data format is incorrect")
	}
	return true, nil
}

// 加密数据，返回base64编码的加密数据
func EncryptData(data interface{}) string {
	input, _ := json.Marshal(data)
	secretData := EncryptString(string(input), "")
	encodedData := base64.StdEncoding.EncodeToString([]byte(secretData))
	return encodedData
}

// 解密base64编码的数据
func DecryptBase64Data(data []byte) []byte {
	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		//不是base64编码
		return nil
	}
	date := time.Now()
	secretKey := GenSecretKey(&date, false)
	DecryptString(string(decodedData), secretKey)
	return DecryptString(string(decodedData), secretKey)
}

// 解密base64编码的json数据
func DecryptJsonBase64Data(data []byte) []byte {
	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		//不是base64编码
		return nil
	}
	return DecryptJSONString(decodedData, "")
}

// 解密json字符串, 解析失败时，会返回空字符
func DecryptJSONString(input []byte, secretKey string) []byte {
	date := time.Now()
	if secretKey == "" {
		secretKey = GenSecretKey(&date, false)
	}
	decrypted := DecryptString(string(input), secretKey)
	if !IsJSON(decrypted) {
		decrypted = DecryptString(string(input), GenSecretKey(&date, true))
		if !IsJSON(decrypted) {
			return nil
		}
	}
	return []byte(decrypted)
}

// 加密字符串
func EncryptString(input string, secretKey string) []byte {
	if secretKey == "" {
		secretKey = GenSecretKey(nil, false)
	}
	n := len(secretKey)
	var encrypted []byte
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		iv := int(secretKey[i%n])
		encryptedCharCode := charCode + iv
		encrypted = append(encrypted, byte(rune(encryptedCharCode)))
	}
	return encrypted
}

// 解密字符串
func DecryptString(input string, secretKey string) []byte {
	if secretKey == "" {
		date := time.Now()
		secretKey = GenSecretKey(&date, false)
	}
	n := len(secretKey)
	var decrypted []byte
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		iv := int(secretKey[i%n])
		decryptedCharCode := charCode - iv
		decrypted = append(decrypted, byte(rune(decryptedCharCode)))
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

func GenSecretKey(date *time.Time, previousMinute bool) string {
	if date == nil {
		now := time.Now()
		date = &now
	}
	if previousMinute {
		*date = GetPreviousMinuteDate(date)
	}
	timestampInSeconds := date.Unix()
	timestampInMinutes := timestampInSeconds - (timestampInSeconds % 60)
	return fmt.Sprint(timestampInMinutes)
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
		decompressedData, err = DecompressedFlateData(compressedData)
	case "gzip":
		decompressedData, err = DecompressGzipData(compressedData)
	case "zlib":
		decompressedData, err = DecompressZlibData(compressedData)
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

func DecompressedFlateData(compressedData []byte) ([]byte, error) {
	// 创建deflate解压缩器
	reader := flate.NewReader(bytes.NewReader(compressedData))
	defer reader.Close()

	return io.ReadAll(reader)
}

func CompressFlateData(data any) ([]byte, error) {
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

func DecompressGzipData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func CompressGzipData(data []byte) ([]byte, error) {
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

func DecompressZlibData(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	zlibReader, err := zlib.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer zlibReader.Close()

	return io.ReadAll(zlibReader)
}

func CompressZlibData(data []byte) ([]byte, error) {
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

func IsJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}
