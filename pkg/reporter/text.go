package reporter

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/songzhibin97/perfinspector/pkg/rules"
)

// GenerateTextReport 生成文本格式的分析报告
func GenerateTextReport(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends, findings []rules.Finding) {
	GenerateTextReportWithContext(groups, trends, findings, nil)
}

// GenerateTextReportWithContext 生成带问题上下文的文本格式分析报告
func GenerateTextReportWithContext(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends, findings []rules.Finding, contexts map[string]*locator.ProblemContext) {
	if len(groups) == 0 {
		fmt.Println("📭 没有找到可分析的 profile 文件")
		return
	}

	fmt.Println("\n" + "═══════════════════════════════════════════════════════════")
	fmt.Println("                    PerfInspector v0.1 分析报告")
	fmt.Println("═══════════════════════════════════════════════════════════")

	for _, group := range groups {
		if len(group.Files) == 0 {
			continue
		}

		fmt.Printf("\n📁 %s 分析 (%d 个文件):\n", group.Type, len(group.Files))
		fmt.Println("───────────────────────────────────────────────────────────")

		for i, file := range group.Files {
			fmt.Printf("  %d. %s\n", i+1, filepath.Base(file.Path))
			fmt.Printf("     ├─ 时间: %s\n", file.Time.UTC().Format(time.RFC3339))
			fmt.Printf("     ├─ 大小: %s\n", formatSize(file.Size))

			// 显示性能指标
			if file.Metrics != nil {
				printMetrics(file.Metrics, group.Type)
			}
		}

		// 显示时间范围
		if len(group.Files) > 1 {
			first := group.Files[0].Time.UTC()
			last := group.Files[len(group.Files)-1].Time.UTC()
			duration := last.Sub(first)
			fmt.Printf("\n  📊 时间范围: %s → %s\n",
				first.Format("2006-01-02 15:04:05"),
				last.Format("2006-01-02 15:04:05"))
			fmt.Printf("  ⏱️  持续时间: %s\n", formatDuration(duration))
		}

		// 显示趋势（仅 R² > 0.7）
		if groupTrends, ok := trends[group.Type]; ok && groupTrends != nil {
			printTrends(groupTrends)
		}
	}

	// 分离单类型发现和联合分析发现
	var singleFindings, crossFindings []rules.Finding
	for _, f := range findings {
		if f.IsCrossAnalysis {
			crossFindings = append(crossFindings, f)
		} else {
			singleFindings = append(singleFindings, f)
		}
	}

	// 显示单类型规则发现
	if len(singleFindings) > 0 {
		fmt.Println("\n═══════════════════════════════════════════════════════════")
		fmt.Println("                        🔍 规则发现")
		fmt.Println("═══════════════════════════════════════════════════════════")

		for i, finding := range singleFindings {
			// 查找对应的 ProblemContext
			var ctx *locator.ProblemContext
			if contexts != nil {
				ctx = contexts[finding.RuleID]
			}
			printFindingWithContext(i+1, finding, ctx)
		}
	}

	// 显示联合分析发现
	if len(crossFindings) > 0 {
		fmt.Println("\n═══════════════════════════════════════════════════════════")
		fmt.Println("                     🔗 联合分析发现")
		fmt.Println("═══════════════════════════════════════════════════════════")

		for i, finding := range crossFindings {
			// 查找对应的 ProblemContext
			var ctx *locator.ProblemContext
			if contexts != nil {
				ctx = contexts[finding.RuleID]
			}
			printFindingWithContext(i+1, finding, ctx)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
}

// printFinding 打印单个发现（向后兼容）
func printFinding(index int, finding rules.Finding) {
	printFindingWithContext(index, finding, nil)
}

// printFindingWithContext 打印单个发现，包含问题上下文
func printFindingWithContext(index int, finding rules.Finding, ctx *locator.ProblemContext) {
	severityIcon := getSeverityIcon(finding.Severity)
	fmt.Printf("\n%d. %s %s\n", index, severityIcon, finding.Title)
	fmt.Printf("   规则: %s (%s)\n", finding.RuleName, finding.RuleID)
	fmt.Printf("   严重程度: %s\n", finding.Severity)

	// 如果有 ProblemContext，显示增强信息
	if ctx != nil {
		// 显示问题解释
		if ctx.Explanation != "" {
			fmt.Println("\n   📝 问题解释:")
			printWrappedText(ctx.Explanation, "      ", 70)
		}

		// 显示影响评估
		if ctx.Impact != "" {
			fmt.Println("\n   📊 影响评估:")
			fmt.Printf("      %s\n", ctx.Impact)
		}

		// 显示热点路径
		if len(ctx.HotPaths) > 0 {
			printHotPaths(ctx.HotPaths)
		}

		// 显示可执行命令
		if len(ctx.Commands) > 0 {
			printCommands(ctx.Commands)
		}

		// 显示建议和代码示例
		if len(ctx.Suggestions) > 0 {
			printSuggestions(ctx.Suggestions)
		}
	} else {
		// 没有 ProblemContext 时，使用原有的显示方式
		if len(finding.Evidence) > 0 {
			fmt.Println("   证据:")
			for key, value := range finding.Evidence {
				fmt.Printf("     - %s: %s\n", key, value)
			}
		}

		if len(finding.Suggestions) > 0 {
			fmt.Println("   建议:")
			for _, suggestion := range finding.Suggestions {
				fmt.Printf("     • %s\n", suggestion)
			}
		}
	}
}

// printTrends 打印趋势信息（仅 R² > 0.7）
func printTrends(trends *analyzer.GroupTrends) {
	printed := false

	if trends.HeapInuse != nil && trends.HeapInuse.R2 > 0.7 {
		if !printed {
			fmt.Println("\n  📈 趋势分析:")
			printed = true
		}
		dirIcon := getDirectionIcon(trends.HeapInuse.Direction)
		fmt.Printf("     %s 堆内存: 斜率=%.2f, R²=%.2f (%s)\n",
			dirIcon, trends.HeapInuse.Slope, trends.HeapInuse.R2, trends.HeapInuse.Direction)
	}

	if trends.GoroutineCount != nil && trends.GoroutineCount.R2 > 0.7 {
		if !printed {
			fmt.Println("\n  📈 趋势分析:")
			printed = true
		}
		dirIcon := getDirectionIcon(trends.GoroutineCount.Direction)
		fmt.Printf("     %s Goroutine: 斜率=%.2f, R²=%.2f (%s)\n",
			dirIcon, trends.GoroutineCount.Slope, trends.GoroutineCount.R2, trends.GoroutineCount.Direction)
	}
}

// getDirectionIcon 获取趋势方向图标
func getDirectionIcon(direction string) string {
	switch direction {
	case "increasing":
		return "📈"
	case "decreasing":
		return "📉"
	default:
		return "➡️"
	}
}

// getSeverityIcon 获取严重程度图标
func getSeverityIcon(severity string) string {
	switch severity {
	case "critical":
		return "🔥"
	case "high":
		return "🔴"
	case "medium":
		return "🟡"
	case "low":
		return "🟢"
	default:
		return "⚪"
	}
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
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

// printMetrics 打印性能指标
func printMetrics(m *analyzer.ProfileMetrics, profileType string) {
	switch profileType {
	case "cpu":
		if m.CPUTime > 0 {
			fmt.Printf("     ├─ CPU时间: %v\n", m.CPUTime)
		}
		if m.Duration > 0 {
			fmt.Printf("     ├─ 采样时长: %v\n", m.Duration)
		}
		fmt.Printf("     ├─ 样本数: %d\n", m.TotalSamples)
		if len(m.TopFunctions) > 0 {
			fmt.Println("     ├─ Top 热点函数:")
			for i, fn := range m.TopFunctions {
				if i >= 5 {
					break
				}
				fmt.Printf("     │  %d. %s (%.1f%%)\n", i+1, truncateName(fn.Name, 50), fn.FlatPct)
			}
		}
		fmt.Println("     └─")

	case "heap":
		fmt.Printf("     ├─ 已分配: %s (%d 对象)\n", analyzer.FormatBytes(m.AllocSpace), m.AllocObjects)
		fmt.Printf("     ├─ 使用中: %s (%d 对象)\n", analyzer.FormatBytes(m.InuseSpace), m.InuseObjects)
		if len(m.TopFunctions) > 0 {
			fmt.Println("     ├─ Top 内存分配点:")
			for i, fn := range m.TopFunctions {
				if i >= 5 {
					break
				}
				fmt.Printf("     │  %d. %s (%.1f%%)\n", i+1, truncateName(fn.Name, 50), fn.FlatPct)
			}
		}
		fmt.Println("     └─")

	case "goroutine":
		fmt.Printf("     ├─ Goroutine数: %d\n", m.GoroutineCount)
		if len(m.TopFunctions) > 0 {
			fmt.Println("     ├─ Top 阻塞点:")
			for i, fn := range m.TopFunctions {
				if i >= 5 {
					break
				}
				fmt.Printf("     │  %d. %s (%d)\n", i+1, truncateName(fn.Name, 50), fn.Flat)
			}
		}
		fmt.Println("     └─")

	default:
		fmt.Printf("     ├─ 样本数: %d\n", m.TotalSamples)
		fmt.Printf("     ├─ 函数数: %d\n", m.NumFunctions)
		fmt.Println("     └─")
	}
}

// truncateName 截断函数名
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return "..." + name[len(name)-maxLen+3:]
}

// printHotPaths 打印热点路径列表
func printHotPaths(hotPaths []locator.HotPath) {
	fmt.Println("\n   🔥 热点调用链:")
	for i, hp := range hotPaths {
		fmt.Printf("\n   ─── 热点 #%d (%.1f%%) ───\n", i+1, hp.Chain.TotalPct)

		// 打印类别分布摘要
		printCategorySummary(hp.Chain)

		// 打印调用链
		printCallChain(hp)
	}
}

// printCallChain 打印带分类标记的调用链
func printCallChain(hp locator.HotPath) {
	frames := hp.Chain.Frames
	if len(frames) == 0 {
		fmt.Println("      (空调用链)")
		return
	}

	// 创建业务帧索引集合，用于快速查找
	businessFrameSet := make(map[int]bool)
	for _, idx := range hp.BusinessFrames {
		businessFrameSet[idx] = true
	}

	var lastCategory locator.CodeCategory
	for i, frame := range frames {
		// 检查是否需要打印类别分隔线
		if i > 0 && frame.Category != lastCategory {
			fmt.Println("      ─────────────────────────────")
		}

		// 获取类别图标
		icon := getCategoryIcon(frame.Category)

		// 判断是否为业务帧（需要高亮）
		highlight := ""
		if businessFrameSet[i] {
			if i == hp.RootCauseIndex {
				highlight = " ← 根因"
			} else {
				highlight = " ← 关注"
			}
		}

		// 打印栈帧
		fmt.Printf("      %s [%s] %s%s\n", icon, frame.Category.String(), frame.ShortName, highlight)
		fmt.Printf("             └─ %s\n", frame.Location())

		lastCategory = frame.Category
	}

	// 如果没有业务代码，显示提示
	if !hp.Chain.HasBusinessCode() {
		fmt.Println("\n      ⚠️  该路径中没有业务代码 - 可能是运行时/GC 问题或间接调用")
	}
}

// getCategoryIcon 返回类别对应的图标
func getCategoryIcon(category locator.CodeCategory) string {
	return category.Icon()
}

// printCategorySummary 打印类别分布摘要
func printCategorySummary(chain locator.CallChain) {
	summary := chain.Summary()
	if summary != "" {
		fmt.Printf("      调用链: %s\n", summary)
	}
}

// printCommands 打印可执行命令
func printCommands(commands []locator.ExecutableCmd) {
	if len(commands) == 0 {
		return
	}

	fmt.Println("\n   💻 调试命令:")
	for i, cmd := range commands {
		fmt.Printf("\n      %d. %s\n", i+1, cmd.Description)
		fmt.Printf("         $ %s\n", cmd.Command)
		if cmd.OutputHint != "" {
			fmt.Printf("         说明: %s\n", cmd.OutputHint)
		}
	}
}

// printSuggestions 打印分类建议
func printSuggestions(suggestions []locator.Suggestion) {
	if len(suggestions) == 0 {
		return
	}

	// 分离立即建议和长期建议
	var immediate, longTerm []locator.Suggestion
	for _, s := range suggestions {
		if s.Category == "long_term" {
			longTerm = append(longTerm, s)
		} else {
			immediate = append(immediate, s)
		}
	}

	fmt.Println("\n   💡 建议:")

	if len(immediate) > 0 {
		fmt.Println("      [立即]")
		for _, s := range immediate {
			fmt.Printf("        • %s\n", s.Content)
		}
	}

	if len(longTerm) > 0 {
		fmt.Println("      [长期]")
		for _, s := range longTerm {
			fmt.Printf("        • %s\n", s.Content)
		}
	}
}

// printWrappedText 打印自动换行的文本
func printWrappedText(text string, prefix string, maxWidth int) {
	// 按换行符分割
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			fmt.Println()
			continue
		}

		// 简单的单词换行
		words := strings.Fields(para)
		if len(words) == 0 {
			fmt.Println(prefix)
			continue
		}

		line := prefix
		lineLen := len(prefix)

		for _, word := range words {
			wordLen := len(word)
			if lineLen+wordLen+1 > maxWidth && lineLen > len(prefix) {
				fmt.Println(line)
				line = prefix + word
				lineLen = len(prefix) + wordLen
			} else {
				if lineLen > len(prefix) {
					line += " "
					lineLen++
				}
				line += word
				lineLen += wordLen
			}
		}

		if lineLen > len(prefix) {
			fmt.Println(line)
		}
	}
}
