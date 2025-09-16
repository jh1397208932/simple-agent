package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/jhUtil/simple-agent-go/internal/util/encodingutil"

	"github.com/gin-gonic/gin"
)

var AesPassword []byte = []byte("meiyoumima")
var AesKey = []byte("*1'Z;XLCZ(*^#^@*()212oawePJ[,23]")
var HmacKey = "tesw-dadad0-pm2-pp9"

type nullResponseWriter struct {
	gin.ResponseWriter
}

func (w *nullResponseWriter) Write(data []byte) (int, error) {
	return 0, nil
}

func (w *nullResponseWriter) WriteHeader(statusCode int) {
	// 完全不做任何操作
}

func (w *nullResponseWriter) Header() http.Header {
	return make(http.Header) // 返回空header
}
func AuthMiddleware(aesPassword string) gin.HandlerFunc {
	if aesPassword != "" {
		AesPassword = []byte(aesPassword)
	}
	return func(c *gin.Context) {
		// 1. 获取加密token
		authHeader := c.GetHeader("Auth")
		timestamp, err := strconv.ParseInt(c.GetHeader("Timestamp"), 10, 64)
		nonce := c.GetHeader("Nonce")
		signature := c.GetHeader("Signature")
		fmt.Printf("timestamp:%d nonce:%s signature:%s", timestamp, nonce, signature)
		if err != nil || authHeader == "" || signature == "" || nonce == "" {
			//c.Writer = &nullResponseWriter{ResponseWriter: c.Writer}
			fmt.Println("鉴权不通过3")
			noResponse(c)
			//c.Abort()
			return
		}
		isValid := encodingutil.VerifyHMACSignature(timestamp, nonce, signature, authHeader, HmacKey, int64(300))

		if !isValid {

			fmt.Println("鉴权不通过1")
			noResponse(c)
			return
		}

		//判断aes凭据
		decoAuthHead, err := encodingutil.AesDecryptGCM(authHeader, AesKey)
		if err != nil {
			//c.Writer = &nullResponseWriter{ResponseWriter: c.Writer}
			//c.Abort()
			fmt.Println("鉴权不通4")
			noResponse(c)
			return
		}
		if string(decoAuthHead) == string(AesPassword) {
			c.Next()
		} else {
			//c.Writer = &nullResponseWriter{ResponseWriter: c.Writer}
			fmt.Println("鉴权不通过5")
			noResponse(c)
			//c.Abort()
			return
		}

	}
}

type hijackWriter struct {
	gin.ResponseWriter
	conn net.Conn
}

func (w *hijackWriter) Write(data []byte) (int, error) {
	return 0, nil // 丢弃所有写入
}

func (w *hijackWriter) WriteHeader(code int) {
	// 禁止写入状态码
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, nil, nil
}

func noResponse(c *gin.Context) {
	fmt.Printf("鉴权不通过22")
	// 1. 获取底层TCP连接
	hj, ok := c.Writer.(http.Hijacker)
	if !ok {
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}

	// 2. 替换Writer并立即关闭连接
	c.Writer = &hijackWriter{
		ResponseWriter: c.Writer,
		conn:           conn,
	}

	// 3. 直接关闭连接（无任何响应）
	conn.Close()
	c.Abort()
}
