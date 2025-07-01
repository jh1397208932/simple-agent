package randUtil

import (
	"crypto/rand"
	"encoding/base32"
	"time"
)

// 生成简单随机数id
func GenShortID() string {
	buf := make([]byte, 10)
	rand.Read(buf)
	// 时间戳(8字符) + 随机数(12字符)
	return time.Now().Format("060102150405") + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(
		buf,
	)[:12] // 输出示例：25062815K4VX7Z9TW2P6
}
