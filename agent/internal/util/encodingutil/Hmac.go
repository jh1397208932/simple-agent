package encodingutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
	// "github.com/google/uuid"
)

// 生成HMAC签名
// 参数: 时间戳、随机UUID、HMAC密钥
// 返回: 十六进制编码的HMAC签名
func GenerateHMACSignature(timestamp int64, nonce string, data string, secretKey string) string {
	// 1. 构建待签名字符串（按约定格式组合参数，这里使用"timestamp:nonce"格式）
	signStr := fmt.Sprintf("%d:%s:%s", timestamp, nonce, data)

	// 2. 创建HMAC实例，使用SHA256哈希算法
	h := hmac.New(sha256.New, []byte(secretKey))

	// 3. 写入待签名数据
	h.Write([]byte(signStr))

	// 4. 计算HMAC值并转为十六进制字符串
	return hex.EncodeToString(h.Sum(nil))
}

// 验证HMAC签名
// 参数: 时间戳、随机UUID、收到的签名、HMAC密钥、签名有效期(秒)
// 返回: 验证是否通过
func VerifyHMACSignature(timestamp int64, nonce string, receivedSign string, data string, secretKey string, maxAge int64) bool {
	// 1. 验证时间戳是否在有效期内（防止重放攻击）
	currentTime := time.Now().Unix()
	if currentTime-timestamp > maxAge || timestamp > currentTime {
		return false // 时间戳过期或未来时间，验证失败
	}

	// 2. 重新生成签名
	generatedSign := GenerateHMACSignature(timestamp, nonce, data, secretKey)

	// 3. 比较生成的签名与收到的签名（使用hmac.Equal防止时序攻击）
	return hmac.Equal([]byte(generatedSign), []byte(receivedSign))
}

// func main() {
// 	// 示例使用
// 	secretKey := "your-secret-key-123" // HMAC密钥（实际应用中需安全保管）
// 	maxAge := int64(300)               // 签名有效期5分钟（300秒）

// 	// 生成必要参数
// 	timestamp := time.Now().Unix()       // 时间戳（秒级）
// 	nonce := uuid.New().String()         // 随机UUID（nonce）

// 	// 生成签名
// 	signature := GenerateHMACSignature(timestamp, nonce, secretKey)
// 	fmt.Printf("生成的签名参数:\n")
// 	fmt.Printf("时间戳: %d\n", timestamp)
// 	fmt.Printf("随机UUID: %s\n", nonce)
// 	fmt.Printf("HMAC签名: %s\n", signature)

// 	// 验证签名（模拟服务端验证过程）
// 	isValid := VerifyHMACSignature(timestamp, nonce, signature, secretKey, maxAge)
// 	if isValid {
// 		fmt.Println("签名验证通过")
// 	} else {
// 		fmt.Println("签名验证失败")
// 	}

// 	// 测试一个无效的签名
// 	invalidSign := "invalid-signature-123"
// 	isValid = VerifyHMACSignature(timestamp, nonce, invalidSign, secretKey, maxAge)
// 	fmt.Printf("验证无效签名结果: %v\n", isValid)
// }
