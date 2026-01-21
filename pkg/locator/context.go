package locator

import (
	"fmt"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/rules"
)

// ContextGenerator 问题上下文生成器
type ContextGenerator struct {
	analyzer *PathAnalyzer
}

// NewContextGenerator 创建生成器
func NewContextGenerator(analyzer *PathAnalyzer) *ContextGenerator {
	return &ContextGenerator{
		analyzer: analyzer,
	}
}

// GenerateContext 生成问题上下文
// 从 Finding 和 profiles 生成完整的 ProblemContext
func (g *ContextGenerator) GenerateContext(
	finding rules.Finding,
	profiles map[string]*profile.Profile,
) *ProblemContext {
	return g.GenerateContextWithPaths(finding, profiles, nil)
}

// GenerateContextWithPaths 生成问题上下文（带 profile 路径）
// 从 Finding、profiles 和 profile 路径生成完整的 ProblemContext
func (g *ContextGenerator) GenerateContextWithPaths(
	finding rules.Finding,
	profiles map[string]*profile.Profile,
	profilePaths []string,
) *ProblemContext {
	return g.GenerateContextWithAllProfiles(finding, profiles, nil, profilePaths)
}

// GenerateContextWithAllProfiles 生成问题上下文（支持多个 profile 综合分析）
// 从 Finding、单个 profile（向后兼容）、所有 profiles 和 profile 路径生成完整的 ProblemContext
func (g *ContextGenerator) GenerateContextWithAllProfiles(
	finding rules.Finding,
	profiles map[string]*profile.Profile,
	allProfiles map[string][]*profile.Profile,
	profilePaths []string,
) *ProblemContext {
	if g.analyzer == nil {
		return nil
	}

	// 确定 profile 类型
	profileType := determineProfileType(finding)

	// 分析热点路径
	var hotPaths []HotPath

	// 优先使用所有 profiles 进行综合分析（特别是 CPU 类型）
	if allProfiles != nil {
		for pType, profs := range allProfiles {
			if strings.Contains(strings.ToLower(pType), profileType) && len(profs) > 0 {
				// 使用多 profile 综合分析
				hotPaths = g.analyzer.AnalyzeMultipleProfiles(profs, profileType)
				break
			}
		}
	}

	// 如果没有使用多 profile 分析，回退到单个 profile
	if len(hotPaths) == 0 && profiles != nil {
		for pType, prof := range profiles {
			if strings.Contains(strings.ToLower(pType), profileType) {
				hotPaths = g.analyzer.AnalyzeHotPaths(prof, profileType)
				break
			}
		}
	}

	// 生成问题上下文
	ctx := &ProblemContext{
		Title:       finding.Title,
		Severity:    normalizeSeverity(finding.Severity),
		Explanation: GenerateExplanation(finding, hotPaths),
		Impact:      GenerateImpact(hotPaths, profileType),
		HotPaths:    hotPaths,
		Commands:    generateCommands(profileType, hotPaths, profilePaths),
		Suggestions: GenerateSuggestions(finding, hotPaths),
	}

	return ctx
}

// determineProfileType 从 Finding 确定 profile 类型
func determineProfileType(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)
	ruleID := strings.ToLower(finding.RuleID)

	// 根据标题或规则 ID 判断类型
	if strings.Contains(title, "cpu") || strings.Contains(ruleID, "cpu") {
		return "cpu"
	}
	if strings.Contains(title, "内存") || strings.Contains(title, "memory") ||
		strings.Contains(title, "heap") || strings.Contains(ruleID, "heap") ||
		strings.Contains(ruleID, "memory") {
		return "heap"
	}
	if strings.Contains(title, "goroutine") || strings.Contains(ruleID, "goroutine") ||
		strings.Contains(title, "协程") {
		return "goroutine"
	}

	// 默认返回 cpu
	return "cpu"
}

// normalizeSeverity 标准化严重程度
func normalizeSeverity(severity string) string {
	s := strings.ToLower(severity)
	switch s {
	case "critical", "严重":
		return "critical"
	case "high", "高":
		return "high"
	case "medium", "中":
		return "medium"
	case "low", "低":
		return "low"
	default:
		return "medium"
	}
}

// GenerateExplanation 生成通俗易懂的问题解释
func GenerateExplanation(finding rules.Finding, hotPaths []HotPath) string {
	if len(hotPaths) == 0 {
		return generateBasicExplanation(finding)
	}

	var sb strings.Builder

	// 基础解释
	sb.WriteString(generateBasicExplanation(finding))

	// 添加热点路径相关的解释
	if len(hotPaths) > 0 {
		topPath := hotPaths[0]

		// 检查是否有业务代码
		if topPath.RootCauseIndex >= 0 && topPath.RootCauseIndex < len(topPath.Chain.Frames) {
			rootCause := topPath.Chain.Frames[topPath.RootCauseIndex]
			sb.WriteString(fmt.Sprintf(" 主要问题出现在业务代码 %s 函数（%s）",
				rootCause.ShortName, rootCause.Location()))

			// 分析业务代码调用了什么
			if topPath.RootCauseIndex < len(topPath.Chain.Frames)-1 {
				// 找到业务代码之后的第一个非业务代码帧
				for i := topPath.RootCauseIndex + 1; i < len(topPath.Chain.Frames); i++ {
					frame := topPath.Chain.Frames[i]
					if frame.Category != CategoryBusiness {
						sb.WriteString(fmt.Sprintf("，该函数调用了 %s (%s)",
							getCategoryDescription(frame.Category), frame.ShortName))
						break
					}
				}
			}
			sb.WriteString("。")
		} else if !topPath.Chain.HasBusinessCode() {
			// 没有业务代码，但可能是业务代码间接触发的
			sb.WriteString(" 该热点路径中没有直接的业务代码，")

			// 分析调用链的组成
			breakdown := topPath.Chain.CategoryBreakdown
			if breakdown[CategoryRuntime] > 0 && breakdown[CategoryRuntime] == len(topPath.Chain.Frames) {
				sb.WriteString("全部是 Go 运行时代码，通常是 GC 或内存管理开销。")
			} else if breakdown[CategoryThirdParty] > 0 {
				sb.WriteString("主要是第三方库调用，可能是业务代码通过第三方库间接触发的。")
			} else if breakdown[CategoryStdlib] > 0 {
				sb.WriteString("主要是标准库调用，可能是业务代码通过标准库间接触发的。")
			} else {
				sb.WriteString("可能是业务代码间接触发的运行时开销。")
			}
		}
	}

	return sb.String()
}

// getCategoryDescription 获取代码类别的描述
func getCategoryDescription(category CodeCategory) string {
	switch category {
	case CategoryRuntime:
		return "Go 运行时"
	case CategoryStdlib:
		return "标准库"
	case CategoryThirdParty:
		return "第三方库"
	case CategoryBusiness:
		return "业务代码"
	default:
		return "未知代码"
	}
}

// generateBasicExplanation 生成基础问题解释
func generateBasicExplanation(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)

	// 根据问题类型生成解释
	if strings.Contains(title, "内存") || strings.Contains(title, "memory") ||
		strings.Contains(title, "heap") {
		return generateMemoryExplanation(finding)
	}
	if strings.Contains(title, "cpu") {
		return generateCPUExplanation(finding)
	}
	if strings.Contains(title, "goroutine") || strings.Contains(title, "协程") {
		return generateGoroutineExplanation(finding)
	}

	// 默认解释
	return fmt.Sprintf("检测到性能问题：%s。建议检查相关代码并进行优化。", finding.Title)
}

// generateMemoryExplanation 生成内存问题解释
func generateMemoryExplanation(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "泄漏") || strings.Contains(title, "leak") ||
		strings.Contains(title, "增长") || strings.Contains(title, "growth") {
		return "你的程序内存使用量在持续增长。这通常意味着存在内存泄漏 - 某些对象被创建后没有被正确释放。常见原因包括：未关闭的资源（文件、连接）、持续增长的 slice/map、缓存没有过期策略等。"
	}

	if strings.Contains(title, "分配") || strings.Contains(title, "alloc") {
		return "程序存在大量内存分配操作。频繁的内存分配会增加 GC 压力，影响程序性能。建议检查是否可以复用对象、使用对象池或减少不必要的分配。"
	}

	return "检测到内存相关问题。建议使用 pprof 工具分析内存分配情况，找出内存消耗的热点。"
}

// generateCPUExplanation 生成 CPU 问题解释
func generateCPUExplanation(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "热点") || strings.Contains(title, "hotspot") ||
		strings.Contains(title, "高") || strings.Contains(title, "high") {
		return "程序存在 CPU 热点，某些函数消耗了大量 CPU 时间。这可能是由于算法效率低下、不必要的计算或循环优化不足导致的。"
	}

	return "检测到 CPU 性能问题。建议分析 CPU profile 找出消耗最多 CPU 时间的函数，并考虑优化算法或减少不必要的计算。"
}

// generateGoroutineExplanation 生成 goroutine 问题解释
func generateGoroutineExplanation(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "泄漏") || strings.Contains(title, "leak") ||
		strings.Contains(title, "增长") || strings.Contains(title, "growth") {
		return "程序的 goroutine 数量在持续增长。这通常意味着存在 goroutine 泄漏 - goroutine 被创建后没有正确退出。常见原因包括：channel 阻塞、未设置超时的网络操作、忘记关闭的 goroutine 等。"
	}

	if strings.Contains(title, "阻塞") || strings.Contains(title, "block") {
		return "检测到 goroutine 阻塞问题。某些 goroutine 可能在等待 channel、锁或 I/O 操作。建议检查是否存在死锁或资源竞争。"
	}

	return "检测到 goroutine 相关问题。建议分析 goroutine profile 了解 goroutine 的状态分布和阻塞原因。"
}

// GenerateImpact 生成影响评估字符串
func GenerateImpact(hotPaths []HotPath, profileType string) string {
	if len(hotPaths) == 0 {
		return "无法评估影响 - 没有找到热点路径"
	}

	var sb strings.Builder

	// 计算总消耗
	totalPct := 0.0
	for _, hp := range hotPaths {
		totalPct += hp.Chain.TotalPct
	}

	// 主要消耗点
	topPath := hotPaths[0]
	topPct := topPath.Chain.TotalPct

	switch profileType {
	case "cpu":
		sb.WriteString(fmt.Sprintf("主要消耗点占用 %.1f%% 的 CPU 时间", topPct))
		if len(hotPaths) > 1 {
			sb.WriteString(fmt.Sprintf("，前 %d 个热点路径共占用 %.1f%% 的 CPU 时间", len(hotPaths), totalPct))
		}
	case "heap":
		sb.WriteString(fmt.Sprintf("主要消耗点占用 %.1f%% 的内存分配", topPct))
		if len(hotPaths) > 1 {
			sb.WriteString(fmt.Sprintf("，前 %d 个热点路径共占用 %.1f%% 的内存", len(hotPaths), totalPct))
		}
	case "goroutine":
		sb.WriteString(fmt.Sprintf("主要消耗点占用 %.1f%% 的 goroutine", topPct))
		if len(hotPaths) > 1 {
			sb.WriteString(fmt.Sprintf("，前 %d 个热点路径共占用 %.1f%% 的 goroutine", len(hotPaths), totalPct))
		}
	default:
		sb.WriteString(fmt.Sprintf("主要消耗点占用 %.1f%%", topPct))
	}

	// 添加根因信息
	if topPath.RootCauseIndex >= 0 && topPath.RootCauseIndex < len(topPath.Chain.Frames) {
		rootCause := topPath.Chain.Frames[topPath.RootCauseIndex]
		sb.WriteString(fmt.Sprintf("。根因位于: %s (%s)", rootCause.ShortName, rootCause.Location()))
	}

	return sb.String()
}

// GenerateSuggestions 生成分类建议列表
func GenerateSuggestions(finding rules.Finding, hotPaths []HotPath) []Suggestion {
	suggestions := make([]Suggestion, 0)

	// 从 Finding 中提取建议（来自规则文件）
	for _, s := range finding.Suggestions {
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  s,
		})
	}

	// 根据热点路径生成额外建议
	if len(hotPaths) > 0 {
		topPath := hotPaths[0]

		// 如果有业务代码帧，生成针对性建议
		if topPath.RootCauseIndex >= 0 && topPath.RootCauseIndex < len(topPath.Chain.Frames) {
			rootCause := topPath.Chain.Frames[topPath.RootCauseIndex]
			suggestions = append(suggestions, Suggestion{
				Category: "immediate",
				Content:  fmt.Sprintf("检查 %s 附近的代码逻辑", rootCause.Location()),
			})
		} else if !topPath.Chain.HasBusinessCode() {
			// 没有业务代码帧，生成通用排查建议
			suggestions = append(suggestions, generateNoBusinessCodeSuggestions(topPath.ProfileType)...)
		}

		// 根据 profile 类型生成长期建议
		suggestions = append(suggestions, generateLongTermSuggestions(topPath.ProfileType)...)
	}

	// 如果没有任何建议，添加通用建议
	if len(suggestions) == 0 {
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "使用 pprof 工具进行详细分析",
		})
	}

	return suggestions
}

// generateNoBusinessCodeSuggestions 生成无业务代码情况的排查建议
func generateNoBusinessCodeSuggestions(profileType string) []Suggestion {
	suggestions := make([]Suggestion, 0)

	switch profileType {
	case "heap":
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "热点路径中没有业务代码，可能是以下原因：",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "1. 全局 map/slice 持续增长（检查全局变量）",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "2. 缓存没有过期策略（检查缓存实现）",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "3. 连接池/对象池泄漏（检查资源管理）",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "使用 go tool pprof -alloc_objects 查看对象分配来源",
		})
	case "goroutine":
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "热点路径中没有业务代码，goroutine 可能阻塞在运行时调用",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "检查是否有未关闭的 channel 或无限等待的 select",
		})
	case "cpu":
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "CPU 消耗主要在运行时，可能是 GC 压力过大",
		})
		suggestions = append(suggestions, Suggestion{
			Category: "immediate",
			Content:  "考虑减少内存分配或使用 sync.Pool 复用对象",
		})
	}

	return suggestions
}

// generateLongTermSuggestions 生成长期建议
func generateLongTermSuggestions(profileType string) []Suggestion {
	suggestions := make([]Suggestion, 0)

	switch profileType {
	case "cpu":
		suggestions = append(suggestions, Suggestion{
			Category: "long_term",
			Content:  "考虑添加 CPU 性能监控告警，定期 review CPU profile",
		})
	case "heap":
		suggestions = append(suggestions, Suggestion{
			Category: "long_term",
			Content:  "添加内存监控告警，定期 review 内存 profile，考虑使用对象池减少分配",
		})
	case "goroutine":
		suggestions = append(suggestions, Suggestion{
			Category: "long_term",
			Content:  "添加 goroutine 数量监控，确保所有 goroutine 都有退出机制",
		})
	}

	return suggestions
}

// generateCommands 生成可执行命令列表
// 使用 CommandGenerator 生成命令
// profilePaths: 实际的 profile 文件路径列表
func generateCommands(profileType string, hotPaths []HotPath, profilePaths []string) []ExecutableCmd {
	generator := NewCommandGenerator()

	// 如果没有提供实际路径，使用默认路径
	if len(profilePaths) == 0 {
		profilePath := fmt.Sprintf("./%s.pprof", profileType)
		return generator.GenerateCommandsForProfileType(profilePath, profileType, hotPaths)
	}

	// 使用新的 GenerateCommandsWithContext 方法
	return generator.GenerateCommandsWithContext(profilePaths, profileType, hotPaths)
}
