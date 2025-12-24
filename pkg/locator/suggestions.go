package locator

// 建议相关的辅助函数
// 建议内容来自规则文件 (assets/default_rules.yaml)

// containsKeyword 检查函数名是否包含关键词
func containsKeyword(funcName, keyword string) bool {
	// 简单的包含检查
	return len(funcName) > 0 && len(keyword) > 0 &&
		(funcName == keyword ||
			len(funcName) >= len(keyword) && containsSubstring(funcName, keyword))
}

// containsSubstring 检查字符串是否包含子串（不区分大小写）
func containsSubstring(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return indexOf(sLower, substrLower) >= 0
}

// toLower 转换为小写
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// indexOf 查找子串位置
func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
