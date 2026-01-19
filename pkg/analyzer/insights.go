package analyzer

import (
	"fmt"
	"strings"
)

// HeapInsight 堆内存分析洞察
type HeapInsight struct {
	Level       string // info, warning, critical
	Title       string // 洞察标题
	Description string // 详细描述
}

// AnalyzeHeapInsights 分析堆内存并生成洞察（只指出问题点，不给建议）
func AnalyzeHeapInsights(metrics *ProfileMetrics) []HeapInsight {
	var insights []HeapInsight

	if metrics == nil {
		return insights
	}

	// 1. 分析 GC 回收率
	if metrics.AllocSpace > 0 {
		gcRate := float64(metrics.AllocSpace-metrics.InuseSpace) / float64(metrics.AllocSpace) * 100

		if gcRate < 50 {
			insights = append(insights, HeapInsight{
				Level:       "critical",
				Title:       "⚠️  GC 回收率过低",
				Description: fmt.Sprintf("GC 回收率仅 %.1f%%，大量内存无法被回收，可能存在内存泄漏", gcRate),
			})
		} else if gcRate < 80 {
			insights = append(insights, HeapInsight{
				Level:       "warning",
				Title:       "💡 GC 回收率偏低",
				Description: fmt.Sprintf("GC 回收率 %.1f%%，建议检查长生命周期对象", gcRate),
			})
		}
	}

	// 2. 分析当前内存使用
	inuseMB := float64(metrics.InuseSpace) / 1024 / 1024
	if inuseMB > 1024 { // > 1GB
		insights = append(insights, HeapInsight{
			Level:       "warning",
			Title:       "📊 当前内存使用较高",
			Description: fmt.Sprintf("当前使用 %.0f MB 内存", inuseMB),
		})
	}

	// 3. 分析累计分配，识别高频分配
	if len(metrics.TopAllocFunctions) > 0 {
		allocGB := float64(metrics.AllocSpace) / 1024 / 1024 / 1024

		if allocGB > 10 { // 累计分配超过 10GB
			topAlloc := metrics.TopAllocFunctions[0]
			insights = append(insights, HeapInsight{
				Level:       "warning",
				Title:       "� 高频内存分配",
				Description: fmt.Sprintf("累计分配 %.1f GB，Top 分配点: %s (%.1f%%)", allocGB, truncateFuncName(topAlloc.Name), topAlloc.FlatPct),
			})
		}
	}

	// 4. 指出 Top 内存占用函数（业务代码）
	if len(metrics.TopFunctions) > 0 {
		topFunc := metrics.TopFunctions[0]
		funcName := topFunc.Name

		// 识别业务代码（非标准库、非第三方库）
		if !strings.Contains(funcName, "runtime.") &&
			!strings.Contains(funcName, "runtime/") &&
			!isStdLib(funcName) &&
			topFunc.FlatPct > 10 { // 占用超过 10%

			insights = append(insights, HeapInsight{
				Level:       "info",
				Title:       "🎯 主要内存占用点",
				Description: fmt.Sprintf("%s 占用 %.1f%% 内存 (%s)", truncateFuncName(funcName), topFunc.FlatPct, FormatBytes(topFunc.Flat)),
			})
		}
	}

	return insights
}

// isStdLib 判断是否是标准库或常见第三方库
func isStdLib(funcName string) bool {
	stdLibs := []string{
		"encoding/json", "encoding/xml", "encoding/",
		"database/sql",
		"net/http", "net/",
		"io/", "bufio", "bytes", "strings",
		"fmt", "log",
		"sync", "time",
		"crypto/", "hash/",
	}

	for _, lib := range stdLibs {
		if strings.Contains(funcName, lib) {
			return true
		}
	}

	// 第三方库特征
	if strings.Contains(funcName, "github.com/") ||
		strings.Contains(funcName, "google.golang.org/") ||
		strings.Contains(funcName, "go.uber.org/") ||
		strings.Contains(funcName, "gopkg.in/") {
		return true
	}

	return false
}

// truncateFuncName 截断函数名，保留关键部分
func truncateFuncName(name string) string {
	if len(name) <= 60 {
		return name
	}

	// 尝试保留包名和函数名
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		return "..." + strings.Join(parts[len(parts)-2:], "/")
	}

	return name[:57] + "..."
}
