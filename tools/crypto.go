package tools

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
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

	decodedJSONData := DecryptJSONString(body, "")
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

// 加密数据
func EncryptData(data interface{}) []byte {
	input, _ := json.Marshal(data)
	return EncryptString(string(input), "")
}

// 解密json数据, 解析失败时，会返回空字符，返回json字符串的[]byte数据
func DecryptJSONString(input []byte, secretKey string) []byte {
	date := time.Now()
	hasKey := true
	if secretKey == "" {
		hasKey = false
		secretKey = GenSecretKey(&date, false)
	}
	decrypted := DecryptString(string(input), secretKey)
	if !IsJSON(decrypted) {
		if hasKey {
			return nil
		}
		decrypted = DecryptString(string(input), GenSecretKey(&date, true))
		if !IsJSON(decrypted) {
			return nil
		}
	}
	return decrypted
}

// 加密
func EncryptString(input string, secretKey string) []byte {
	if secretKey == "" {
		secretKey = GenSecretKey(nil, false)
	}
	secretKeyRuneArr := []rune(secretKey)
	n := len(secretKeyRuneArr)
	inputRuneArr := []rune(input)
	m := len(inputRuneArr)
	var charCodeArr []string
	for i := 0; i < m; i++ {
		//获取Unicode
		charCode := int(inputRuneArr[i])
		iv := int(secretKeyRuneArr[i%n])
		newCharCode := charCode + iv
		charCodeArr = append(charCodeArr, strconv.Itoa(newCharCode))
	}
	//得到unicode数组字符串
	encryptStr := strings.Join(charCodeArr, ",")
	//编码成base64
	return EncodeBase64(encryptStr)
}

// 解密
func DecryptString(input string, secretKey string) []byte {
	if secretKey == "" {
		secretKey = GenSecretKey(nil, false)
	}
	secretKeyRuneArr := []rune(secretKey)
	n := len(secretKeyRuneArr)
	//先解码base64字符
	input = DecodeBase64(input)
	if input == "" {
		//解码失败
		return nil
	}
	inputRuneArr := strings.Split(input, ",")
	m := len(inputRuneArr)
	var runeArr []rune
	for i := 0; i < m; i++ {
		//获取Unicode
		charCode, err := strconv.Atoi(inputRuneArr[i])
		if err != nil {
			return nil
		}
		iv := int(secretKeyRuneArr[i%n])
		charCode = charCode - iv
		runeArr = append(runeArr, rune(charCode))
	}
	return []byte(string(runeArr))
}

func EncodeBase64(input string) []byte {
	base64Str := base64.StdEncoding.EncodeToString([]byte(input))
	return []byte(base64Str)
}

func DecodeBase64(base64Str string) string {
	decodedStr, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return ""
	}
	return string(decodedStr)
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
	s := strconv.FormatInt(timestampInMinutes*3/10, 10)
	return ReverseString(s) + s
}

func ReverseString(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
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

func MD5(s string) string {
	md5Hash := md5.Sum([]byte(s))
	md5String := hex.EncodeToString(md5Hash[:])
	return md5String
}
