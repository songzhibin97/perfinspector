package rules

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"gopkg.in/yaml.v3"
)

// Engine 规则引擎
type Engine struct {
	rules              []Rule
	crossAnalysisRules []CrossAnalysisRule
}

// NewEngine 创建规则引擎，从指定路径加载规则
func NewEngine(rulesPath string) (*Engine, error) {
	if rulesPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("rules file not found: %s", rulesPath)
		}
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse rules file: %w", err)
	}

	// 验证单类型规则结构
	for i, rule := range config.Rules {
		if rule.ID == "" {
			return nil, fmt.Errorf("rule %d: missing id", i)
		}
		if rule.Name == "" {
			return nil, fmt.Errorf("rule %s: missing name", rule.ID)
		}
		if len(rule.ProfileTypes) == 0 {
			return nil, fmt.Errorf("rule %s: missing profile_types", rule.ID)
		}
		if rule.Condition == "" {
			return nil, fmt.Errorf("rule %s: missing condition", rule.ID)
		}
		if len(rule.Actions) == 0 {
			return nil, fmt.Errorf("rule %s: missing actions", rule.ID)
		}
	}

	// 验证联合分析规则结构
	for i, rule := range config.CrossAnalysisRules {
		if rule.ID == "" {
			return nil, fmt.Errorf("cross_analysis_rule %d: missing id", i)
		}
		if rule.Name == "" {
			return nil, fmt.Errorf("cross_analysis_rule %s: missing name", rule.ID)
		}
		if len(rule.Conditions) < 2 {
			return nil, fmt.Errorf("cross_analysis_rule %s: need at least 2 conditions for cross analysis", rule.ID)
		}
		if len(rule.Actions) == 0 {
			return nil, fmt.Errorf("cross_analysis_rule %s: missing actions", rule.ID)
		}
	}

	return &Engine{
		rules:              config.Rules,
		crossAnalysisRules: config.CrossAnalysisRules,
	}, nil
}

// Evaluate 评估规则，返回匹配的发现
func (e *Engine) Evaluate(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends) []Finding {
	if e == nil {
		return nil
	}

	var findings []Finding

	// 1. 单类型规则评估
	if len(e.rules) > 0 {
		for _, group := range groups {
			groupTrends := trends[group.Type]

			for _, rule := range e.rules {
				// 检查规则是否适用于当前 profile 类型
				if !e.matchesProfileType(rule, group.Type) {
					continue
				}

				// 评估条件
				if e.evaluateCondition(rule.Condition, group, groupTrends) {
					for _, action := range rule.Actions {
						finding := Finding{
							RuleID:      rule.ID,
							RuleName:    rule.Name,
							Severity:    action.Severity,
							Title:       action.Title,
							Evidence:    e.buildEvidence(action.EvidenceTemplate, groupTrends, group),
							Suggestions: action.Suggestions,
						}
						findings = append(findings, finding)
					}
				}
			}
		}
	}

	// 2. 联合分析规则评估
	if len(e.crossAnalysisRules) > 0 {
		crossFindings := e.evaluateCrossAnalysis(groups, trends)
		findings = append(findings, crossFindings...)
	}

	// 3. 去重：合并相同 RuleID 的发现，避免信息冗余
	findings = e.deduplicateFindings(findings)

	return findings
}

// deduplicateFindings 去重发现，合并相同或相似的发现
// 注意：联合分析规则（IsCrossAnalysis=true）优先级更高，不会被单类型规则去重
func (e *Engine) deduplicateFindings(findings []Finding) []Finding {
	if len(findings) <= 1 {
		return findings
	}

	// 分离联合分析规则和单类型规则
	var crossFindings []Finding
	var singleFindings []Finding
	for _, f := range findings {
		if f.IsCrossAnalysis {
			crossFindings = append(crossFindings, f)
		} else {
			singleFindings = append(singleFindings, f)
		}
	}

	// 使用 map 按 RuleID 去重
	seen := make(map[string]bool)
	// 使用 map 按标题关键词去重（处理内容相似的发现）
	seenTitleKeywords := make(map[string]bool)
	result := make([]Finding, 0, len(findings))

	// 优先处理联合分析规则（它们提供更全面的分析）
	for _, finding := range crossFindings {
		key := finding.RuleID + ":" + finding.Title
		if seen[key] {
			continue
		}

		seen[key] = true
		// 联合分析规则标记其涉及的所有关键词
		for _, keyword := range extractAllTitleKeywords(finding.Title) {
			seenTitleKeywords[keyword] = true
		}
		result = append(result, finding)
	}

	// 然后处理单类型规则
	for _, finding := range singleFindings {
		key := finding.RuleID + ":" + finding.Title
		if seen[key] {
			continue
		}

		// 提取标题关键词进行相似性检测
		titleKeyword := extractTitleKeyword(finding.Title)
		// 如果联合分析规则已经覆盖了这个关键词，跳过单类型规则
		// 但如果是不同类型的问题（如 goroutine 和 memory 分开报告），则保留
		if titleKeyword != "" && seenTitleKeywords[titleKeyword] {
			// 检查是否有对应的联合分析规则已经覆盖
			// 如果联合分析规则已经报告了 goroutine+memory 的问题，
			// 则跳过单独的 goroutine 或 memory 规则
			continue
		}

		seen[key] = true
		if titleKeyword != "" {
			seenTitleKeywords[titleKeyword] = true
		}
		result = append(result, finding)
	}

	return result
}

// extractTitleKeyword 提取标题的核心关键词用于相似性检测
func extractTitleKeyword(title string) string {
	// 定义关键词映射，将相似的标题归类
	keywordPatterns := map[string][]string{
		"memory_leak":    {"内存增长", "内存泄漏", "memory leak", "memory growth"},
		"goroutine_leak": {"goroutine", "协程泄漏", "协程增长"},
		"cpu_hotspot":    {"cpu", "热点函数", "cpu hotspot"},
	}

	titleLower := toLowerString(title)
	for keyword, patterns := range keywordPatterns {
		for _, pattern := range patterns {
			if containsString(titleLower, toLowerString(pattern)) {
				return keyword
			}
		}
	}
	return ""
}

// extractAllTitleKeywords 提取标题中的所有关键词（用于联合分析规则）
func extractAllTitleKeywords(title string) []string {
	keywordPatterns := map[string][]string{
		"memory_leak":    {"内存增长", "内存泄漏", "memory leak", "memory growth"},
		"goroutine_leak": {"goroutine", "协程泄漏", "协程增长"},
		"cpu_hotspot":    {"cpu", "热点函数", "cpu hotspot"},
	}

	titleLower := toLowerString(title)
	var keywords []string
	for keyword, patterns := range keywordPatterns {
		for _, pattern := range patterns {
			if containsString(titleLower, toLowerString(pattern)) {
				keywords = append(keywords, keyword)
				break // 每个关键词只添加一次
			}
		}
	}
	return keywords
}

// toLowerString 转换字符串为小写
func toLowerString(s string) string {
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

// containsString 检查字符串是否包含子串
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// evaluateCrossAnalysis 评估联合分析规则
func (e *Engine) evaluateCrossAnalysis(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends) []Finding {
	var findings []Finding

	// 构建 group 类型到 group 的映射
	groupMap := make(map[string]analyzer.ProfileGroup)
	for _, g := range groups {
		groupMap[g.Type] = g
	}

	for _, rule := range e.crossAnalysisRules {
		// 检查所有需要的 profile 类型是否都存在
		allTypesPresent := true
		for profileType := range rule.Conditions {
			if _, exists := groupMap[profileType]; !exists {
				allTypesPresent = false
				break
			}
			if _, exists := trends[profileType]; !exists {
				allTypesPresent = false
				break
			}
		}

		if !allTypesPresent {
			continue
		}

		// 评估每个类型的条件
		allConditionsMet := true
		matchedTrends := make(map[string]*analyzer.TrendMetrics)

		for profileType, condition := range rule.Conditions {
			group := groupMap[profileType]
			groupTrends := trends[profileType]

			if !e.evaluateCrossCondition(condition, profileType, group, groupTrends, matchedTrends) {
				allConditionsMet = false
				break
			}
		}

		if !allConditionsMet {
			continue
		}

		// 检查关联条件
		if rule.Correlation != "" && !e.checkCorrelation(rule.Correlation, matchedTrends) {
			continue
		}

		// 所有条件满足，生成发现
		for _, action := range rule.Actions {
			finding := Finding{
				RuleID:          rule.ID,
				RuleName:        rule.Name,
				Severity:        action.Severity,
				Title:           action.Title,
				Evidence:        e.buildCrossEvidence(action.EvidenceTemplate, trends, groupMap),
				Suggestions:     action.Suggestions,
				IsCrossAnalysis: true,
			}
			findings = append(findings, finding)
		}
	}

	return findings
}

// evaluateCrossCondition 评估联合分析中单个类型的条件
func (e *Engine) evaluateCrossCondition(condition string, profileType string, group analyzer.ProfileGroup, trends *analyzer.GroupTrends, matchedTrends map[string]*analyzer.TrendMetrics) bool {
	if trends == nil {
		return false
	}

	// 需要至少 3 个文件才能做趋势分析
	if len(group.Files) < 3 {
		return false
	}

	switch profileType {
	case "heap":
		if trends.HeapInuse != nil {
			if e.evaluateTrendCondition(condition, trends.HeapInuse) {
				matchedTrends["heap"] = trends.HeapInuse
				return true
			}
		}
	case "goroutine":
		if trends.GoroutineCount != nil {
			if e.evaluateTrendCondition(condition, trends.GoroutineCount) {
				matchedTrends["goroutine"] = trends.GoroutineCount
				return true
			}
		}
	case "cpu":
		// CPU 目前没有趋势分析，检查是否有 CPU 数据
		// 简化实现：检查是否有 CPU 数据
		if len(group.Files) > 0 {
			matchedTrends["cpu"] = &analyzer.TrendMetrics{Direction: "present"}
			return contains(condition, "cpu")
		}
	}

	return false
}

// evaluateTrendCondition 评估趋势条件
func (e *Engine) evaluateTrendCondition(condition string, trend *analyzer.TrendMetrics) bool {
	// 解析条件中的关键词

	// 检查方向条件
	if contains(condition, "increasing") {
		if trend.Direction != "increasing" {
			return false
		}
	}
	if contains(condition, "decreasing") {
		if trend.Direction != "decreasing" {
			return false
		}
	}

	// 检查斜率条件
	if contains(condition, "slope > 0") {
		if trend.Slope <= 0 || trend.R2 < 0.7 {
			return false
		}
	}
	if contains(condition, "slope <= 0") {
		// 斜率小于等于0，或者 R² 太低（趋势不明显）
		if trend.Slope > 0 && trend.R2 > 0.7 {
			return false
		}
	}
	if contains(condition, "slope < 0") {
		if trend.Slope >= 0 {
			return false
		}
	}

	// 如果只是检查 slope 存在（没有比较符号）
	if contains(condition, "slope") && !contains(condition, "slope >") && !contains(condition, "slope <") && !contains(condition, "slope =") {
		if trend.R2 < 0.7 {
			return false
		}
	}

	return true
}

// checkCorrelation 检查关联条件
func (e *Engine) checkCorrelation(correlation string, matchedTrends map[string]*analyzer.TrendMetrics) bool {
	switch correlation {
	case "same_direction":
		// 检查所有趋势方向是否一致
		var direction string
		for _, trend := range matchedTrends {
			if trend.Direction == "present" {
				continue // 跳过没有方向的（如 CPU）
			}
			if direction == "" {
				direction = trend.Direction
			} else if direction != trend.Direction {
				return false
			}
		}
		return direction != ""

	case "both_increasing":
		// 检查是否都在增长
		for _, trend := range matchedTrends {
			if trend.Direction != "increasing" && trend.Direction != "present" {
				return false
			}
		}
		return true

	case "time_correlated":
		// 时间相关性检查（简化版：只要同时存在数据就认为相关）
		return len(matchedTrends) >= 2

	default:
		// 未知关联类型，默认通过
		return true
	}
}

// buildCrossEvidence 构建联合分析的证据
func (e *Engine) buildCrossEvidence(template map[string]string, trends map[string]*analyzer.GroupTrends, groupMap map[string]analyzer.ProfileGroup) map[string]string {
	if template == nil {
		return nil
	}

	evidence := make(map[string]string)
	for key, tmpl := range template {
		value := tmpl

		// 替换 heap 相关变量
		if heapTrends, ok := trends["heap"]; ok && heapTrends != nil && heapTrends.HeapInuse != nil {
			heapGroup := groupMap["heap"]
			durationMinutes := e.calculateDurationMinutes(heapGroup)

			slopePerMinute := 0.0
			if durationMinutes > 0 && len(heapGroup.Files) > 1 {
				totalChange := heapTrends.HeapInuse.Slope * float64(len(heapGroup.Files)-1)
				slopePerMinute = (totalChange / durationMinutes) / (1024 * 1024)
			}

			value = strings.ReplaceAll(value, "{{.heap_slope}}", formatMemoryRate(slopePerMinute))
			value = strings.ReplaceAll(value, "{{.heap_r2}}", fmt.Sprintf("%.2f", heapTrends.HeapInuse.R2))
			value = strings.ReplaceAll(value, "{{.heap_direction}}", heapTrends.HeapInuse.Direction)
		}

		// 替换 goroutine 相关变量
		if goroutineTrends, ok := trends["goroutine"]; ok && goroutineTrends != nil && goroutineTrends.GoroutineCount != nil {
			goroutineGroup := groupMap["goroutine"]
			durationMinutes := e.calculateDurationMinutes(goroutineGroup)

			slopePerMinute := 0.0
			if durationMinutes > 0 && len(goroutineGroup.Files) > 1 {
				totalChange := goroutineTrends.GoroutineCount.Slope * float64(len(goroutineGroup.Files)-1)
				slopePerMinute = totalChange / durationMinutes
			}

			value = strings.ReplaceAll(value, "{{.goroutine_slope}}", fmt.Sprintf("%.2f", slopePerMinute))
			value = strings.ReplaceAll(value, "{{.goroutine_r2}}", fmt.Sprintf("%.2f", goroutineTrends.GoroutineCount.R2))
			value = strings.ReplaceAll(value, "{{.goroutine_direction}}", goroutineTrends.GoroutineCount.Direction)
		}

		evidence[key] = value
	}

	return evidence
}

// calculateDurationMinutes 计算 profile 组的时间跨度（分钟）
func (e *Engine) calculateDurationMinutes(group analyzer.ProfileGroup) float64 {
	if len(group.Files) < 2 {
		return 1
	}
	first := group.Files[0].Time
	last := group.Files[len(group.Files)-1].Time
	minutes := last.Sub(first).Minutes()
	if minutes <= 0 {
		return 1
	}
	return minutes
}

// matchesProfileType 检查规则是否匹配指定的 profile 类型
func (e *Engine) matchesProfileType(rule Rule, profileType string) bool {
	for _, pt := range rule.ProfileTypes {
		if pt == profileType {
			return true
		}
	}
	return false
}

// evaluateCondition 评估规则条件（简化版实现）
func (e *Engine) evaluateCondition(condition string, group analyzer.ProfileGroup, trends *analyzer.GroupTrends) bool {
	// 简化版条件评估：检查趋势是否存在且显著
	// 完整版应该实现表达式解析器

	// CPU 热点分析：只要有 CPU profile 文件就触发
	if condition == "cpu_profile_exists" && group.Type == "cpu" {
		return len(group.Files) > 0
	}

	if trends == nil {
		return false
	}

	// 检查内存增长趋势
	if trends.HeapInuse != nil && trends.HeapInuse.R2 > 0.85 && trends.HeapInuse.Slope > 10.0 {
		if contains(condition, "heap_inuse") && contains(condition, "slope") {
			// 额外检查：确保有足够的文件数量进行趋势分析
			if len(group.Files) >= 3 {
				return true
			}
		}
	}

	// 检查 goroutine 增长趋势
	if trends.GoroutineCount != nil && trends.GoroutineCount.R2 > 0.9 && trends.GoroutineCount.Slope > 1.0 {
		if contains(condition, "goroutine_count") && contains(condition, "slope") {
			// 额外检查：确保有足够的文件数量进行趋势分析
			if len(group.Files) >= 3 {
				return true
			}
		}
	}

	return false
}

// buildEvidence 构建证据数据，替换模板变量
func (e *Engine) buildEvidence(template map[string]string, trends *analyzer.GroupTrends, group analyzer.ProfileGroup) map[string]string {
	if template == nil || trends == nil {
		return nil
	}

	// 计算时间间隔（用于速率转换）
	var durationMinutes float64
	if len(group.Files) > 1 {
		first := group.Files[0].Time
		last := group.Files[len(group.Files)-1].Time
		durationMinutes = last.Sub(first).Minutes()
		if durationMinutes <= 0 {
			durationMinutes = 1 // 避免除零
		}
	}

	evidence := make(map[string]string)
	for key, tmpl := range template {
		value := tmpl

		// 替换堆内存趋势相关变量
		if trends.HeapInuse != nil {
			// 斜率单位是 bytes/样本点，转换为 MB/分钟
			// 计算方式：(斜率 * 样本数) / 时间(分钟) / (1024*1024)
			slopePerMinute := 0.0
			if durationMinutes > 0 && len(group.Files) > 1 {
				// 总变化量 = 斜率 * (样本数-1)
				totalChange := trends.HeapInuse.Slope * float64(len(group.Files)-1)
				// 转换为 MB/分钟
				slopePerMinute = (totalChange / durationMinutes) / (1024 * 1024)
			}
			value = strings.ReplaceAll(value, "{{.slope}}", formatMemoryRate(slopePerMinute))
			value = strings.ReplaceAll(value, "{{.r2}}", fmt.Sprintf("%.2f", trends.HeapInuse.R2))
			value = strings.ReplaceAll(value, "{{.direction}}", trends.HeapInuse.Direction)
		}

		// 替换 Goroutine 趋势相关变量
		if trends.GoroutineCount != nil {
			// Goroutine 斜率转换为 个/分钟
			slopePerMinute := 0.0
			if durationMinutes > 0 && len(group.Files) > 1 {
				totalChange := trends.GoroutineCount.Slope * float64(len(group.Files)-1)
				slopePerMinute = totalChange / durationMinutes
			}
			value = strings.ReplaceAll(value, "{{.goroutine_slope}}", fmt.Sprintf("%.2f", slopePerMinute))
			value = strings.ReplaceAll(value, "{{.goroutine_r2}}", fmt.Sprintf("%.2f", trends.GoroutineCount.R2))
			value = strings.ReplaceAll(value, "{{.goroutine_direction}}", trends.GoroutineCount.Direction)
		}

		// 替换时间范围相关变量
		if len(group.Files) > 1 {
			first := group.Files[0].Time
			last := group.Files[len(group.Files)-1].Time
			duration := last.Sub(first)
			value = strings.ReplaceAll(value, "{{.duration}}", formatDuration(duration))
			value = strings.ReplaceAll(value, "{{.start_time}}", first.Format(time.RFC3339))
			value = strings.ReplaceAll(value, "{{.end_time}}", last.Format(time.RFC3339))
		}

		// 替换文件数量
		value = strings.ReplaceAll(value, "{{.file_count}}", fmt.Sprintf("%d", len(group.Files)))

		evidence[key] = value
	}
	return evidence
}

// formatMemoryRate 格式化内存增长速率，自动选择合适的单位
func formatMemoryRate(mbPerMinute float64) string {
	if mbPerMinute < 0 {
		// 负增长
		return fmt.Sprintf("%.2f", mbPerMinute)
	}
	if mbPerMinute < 1 {
		// 小于 1 MB/分钟，用 KB/分钟
		return fmt.Sprintf("%.2f KB", mbPerMinute*1024)
	}
	if mbPerMinute < 1024 {
		// MB/分钟
		return fmt.Sprintf("%.2f MB", mbPerMinute)
	}
	// GB/分钟
	return fmt.Sprintf("%.2f GB", mbPerMinute/1024)
}

// formatDuration 格式化持续时间
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f 秒", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1f 分钟", d.Minutes())
	}
	return fmt.Sprintf("%.1f 小时", d.Hours())
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
