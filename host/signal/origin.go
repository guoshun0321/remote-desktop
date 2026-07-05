package signal

import "strings"

// originAllowed 判断实际请求 Origin 是否在允许列表内。
//
// 允许列表项支持两种形式:
//   - 精确匹配:"https://app.example.com" 只匹配同串。
//   - 端口通配:"http://localhost:*" 匹配 "http://localhost:<任意端口>",
//     用于本地开发(同一前端框架可能用不同端口)。
//
// 通配只识别结尾的 ":*" —— 简单够用,不做完整 glob。空 origin 一律拒绝
// (浏览器跨域 WS 总会带 Origin;缺失视为可疑)。
//
// 这是纯函数,无副作用,表驱动测试覆盖(origin_test.go)。
func originAllowed(allowed []string, origin string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if a == origin {
			return true
		}
		// 结尾 ":*" 通配:前缀(去掉 ":*" 的部分)匹配 origin 的 host:scheme 段。
		// 例:allowed="http://localhost:*",前缀="http://localhost:",
		// 匹配 "http://localhost:5173" / "http://localhost:3000"。
		if strings.HasSuffix(a, ":*") {
			prefix := a[:len(a)-1] // 保留末尾冒号 → "http://localhost:"
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
	}
	return false
}
