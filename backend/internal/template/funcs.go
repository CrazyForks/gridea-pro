package template

import (
	htmltemplate "html/template"
	"path"
	"strings"
	"time"
)

// TemplateFuncs 返回模板可用的自定义函数
func TemplateFuncs() htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		// 安全 HTML 输出（不转义）
		"safeHTML": func(s string) htmltemplate.HTML {
			return htmltemplate.HTML(s)
		},

		// 安全 CSS 输出
		"safeCSS": func(s string) htmltemplate.CSS {
			return htmltemplate.CSS(s)
		},

		// 安全 JS 输出
		"safeJS": func(s string) htmltemplate.JS {
			return htmltemplate.JS(s)
		},

		// 安全 URL 输出
		"safeURL": func(s string) htmltemplate.URL {
			return htmltemplate.URL(s)
		},

		// or 函数 - 返回第一个非空值
		"or": func(values ...interface{}) interface{} {
			for _, v := range values {
				if v != nil && v != "" && v != false && v != 0 {
					return v
				}
			}
			if len(values) > 0 {
				return values[len(values)-1]
			}
			return nil
		},

		// 日期格式化
		"formatDate": func(t time.Time, format string) string {
			// 将常见格式转换为 Go 格式
			format = convertDateFormat(format)
			return t.Format(format)
		},

		// 字符串连接
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},

		// 默认值
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},

		// 截断字符串
		"truncate": func(length int, s string) string {
			r := []rune(s)
			if len(r) <= length {
				return s
			}
			return string(r[:length]) + "..."
		},

		// 检查切片是否为空
		"empty": func(v interface{}) bool {
			if v == nil {
				return true
			}
			switch val := v.(type) {
			case string:
				return val == ""
			case []interface{}:
				return len(val) == 0
			case []string:
				return len(val) == 0
			}
			return false
		},

		// 不为空
		"notEmpty": func(v interface{}) bool {
			if v == nil {
				return false
			}
			switch val := v.(type) {
			case string:
				return val != ""
			case []interface{}:
				return len(val) > 0
			case []string:
				return len(val) > 0
			case bool:
				return val
			}
			return true
		},

		// 获取当前时间戳（用于缓存刷新）
		"now": func() int64 {
			return time.Now().Unix()
		},

		// URL 路径拼接
		"urlJoin": func(parts ...string) string {
			return path.Join(parts...)
		},
	}
}

// convertDateFormat 将通用日期格式转换为 Go 格式
func convertDateFormat(format string) string {
	// 必须按长度降序排列，防止 "YY" 匹配到 "YYYY"
	replacer := strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MM", "01",
		"DD", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
		"M", "1",
		"D", "2",
	)
	return replacer.Replace(format)
}
