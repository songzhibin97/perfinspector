package reporter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/songzhibin97/perfinspector/pkg/rules"
	"github.com/stretchr/testify/assert"
)

// captureOutput 捕获标准输出
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestGetCategoryIcon 测试类别图标正确性
// **Validates: Requirements 7.2**
func TestGetCategoryIcon(t *testing.T) {
	tests := []struct {
		category locator.CodeCategory
		expected string
	}{
		{locator.CategoryRuntime, "⚙️"},
		{locator.CategoryStdlib, "📚"},
		{locator.CategoryThirdParty, "📦"},
		{locator.CategoryBusiness, "💼"},
		{locator.CategoryUnknown, "❓"},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			icon := getCategoryIcon(tt.category)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

// TestPrintCallChain_WithBusinessFrames 测试带业务帧的调用链格式化输出
// **Validates: Requirements 7.1, 7.2**
func TestPrintCallChain_WithBusinessFrames(t *testing.T) {
	hp := locator.HotPath{
		Chain: locator.CallChain{
			Frames: []locator.StackFrame{
				{
					FunctionName: "main.main",
					ShortName:    "main",
					PackageName:  "main",
					FilePath:     "main.go",
					LineNumber:   10,
					Category:     locator.CategoryBusiness,
				},
				{
					FunctionName: "myapp/handler.HandleRequest",
					ShortName:    "HandleRequest",
					PackageName:  "myapp/handler",
					FilePath:     "handler/request.go",
					LineNumber:   45,
					Category:     locator.CategoryBusiness,
				},
				{
					FunctionName: "net/http.(*Server).Serve",
					ShortName:    "Serve",
					PackageName:  "net/http",
					FilePath:     "net/http/server.go",
					LineNumber:   3000,
					Category:     locator.CategoryStdlib,
				},
				{
					FunctionName: "runtime.mallocgc",
					ShortName:    "mallocgc",
					PackageName:  "runtime",
					FilePath:     "runtime/malloc.go",
					LineNumber:   1234,
					Category:     locator.CategoryRuntime,
				},
			},
			TotalValue:  1000,
			TotalPct:    45.5,
			SampleCount: 10,
			CategoryBreakdown: map[locator.CodeCategory]int{
				locator.CategoryBusiness: 2,
				locator.CategoryStdlib:   1,
				locator.CategoryRuntime:  1,
			},
		},
		BusinessFrames: []int{0, 1},
		RootCauseIndex: 1,
		ProfileType:    "cpu",
	}

	output := captureOutput(func() {
		printCallChain(hp)
	})

	// 验证业务帧被标记
	assert.Contains(t, output, "关注")
	assert.Contains(t, output, "根因")

	// 验证类别图标
	assert.Contains(t, output, "💼")  // business
	assert.Contains(t, output, "📚")  // stdlib
	assert.Contains(t, output, "⚙️") // runtime

	// 验证函数名
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "HandleRequest")
	assert.Contains(t, output, "Serve")
	assert.Contains(t, output, "mallocgc")

	// 验证文件位置
	assert.Contains(t, output, "main.go:10")
	assert.Contains(t, output, "handler/request.go:45")

	// 验证类别分隔线存在（当类别变化时）
	assert.Contains(t, output, "─────────────────────────────")
}

// TestPrintCallChain_NoBusinessCode 测试没有业务代码的调用链
// **Validates: Requirements 7.1**
func TestPrintCallChain_NoBusinessCode(t *testing.T) {
	hp := locator.HotPath{
		Chain: locator.CallChain{
			Frames: []locator.StackFrame{
				{
					FunctionName: "runtime.gcBgMarkWorker",
					ShortName:    "gcBgMarkWorker",
					PackageName:  "runtime",
					FilePath:     "runtime/mgc.go",
					LineNumber:   567,
					Category:     locator.CategoryRuntime,
				},
				{
					FunctionName: "runtime.systemstack",
					ShortName:    "systemstack",
					PackageName:  "runtime",
					FilePath:     "runtime/asm_amd64.s",
					LineNumber:   383,
					Category:     locator.CategoryRuntime,
				},
			},
			TotalValue:  500,
			TotalPct:    20.0,
			SampleCount: 5,
			CategoryBreakdown: map[locator.CodeCategory]int{
				locator.CategoryRuntime: 2,
			},
		},
		BusinessFrames: []int{},
		RootCauseIndex: -1,
		ProfileType:    "cpu",
	}

	output := captureOutput(func() {
		printCallChain(hp)
	})

	// 验证显示无业务代码提示
	assert.Contains(t, output, "没有业务代码")
	assert.Contains(t, output, "运行时/GC 问题")
}

// TestPrintCallChain_EmptyChain 测试空调用链
func TestPrintCallChain_EmptyChain(t *testing.T) {
	hp := locator.HotPath{
		Chain: locator.CallChain{
			Frames: []locator.StackFrame{},
		},
		BusinessFrames: []int{},
		RootCauseIndex: -1,
	}

	output := captureOutput(func() {
		printCallChain(hp)
	})

	assert.Contains(t, output, "空调用链")
}

// TestPrintCategorySummary 测试类别分布摘要
// **Validates: Requirements 7.1**
func TestPrintCategorySummary(t *testing.T) {
	chain := locator.CallChain{
		Frames: []locator.StackFrame{
			{Category: locator.CategoryBusiness},
			{Category: locator.CategoryBusiness},
			{Category: locator.CategoryStdlib},
			{Category: locator.CategoryRuntime},
			{Category: locator.CategoryRuntime},
			{Category: locator.CategoryRuntime},
		},
	}

	output := captureOutput(func() {
		printCategorySummary(chain)
	})

	// 验证摘要格式
	assert.Contains(t, output, "调用链:")
	assert.Contains(t, output, "业务")
	assert.Contains(t, output, "标准库")
	assert.Contains(t, output, "运行时")
	assert.Contains(t, output, "→")
}

// TestPrintCommands 测试命令输出
// **Validates: Requirements 7.6**
func TestPrintCommands(t *testing.T) {
	commands := []locator.ExecutableCmd{
		{
			Command:     "go tool pprof -focus=HandleRequest ./cpu.pprof",
			Description: "聚焦到问题函数",
			OutputHint:  "显示包含 HandleRequest 的调用路径",
		},
		{
			Command:     "go tool pprof -top ./cpu.pprof",
			Description: "查看热点函数排名",
			OutputHint:  "显示消耗最多资源的函数列表",
		},
	}

	output := captureOutput(func() {
		printCommands(commands)
	})

	// 验证命令标题
	assert.Contains(t, output, "调试命令")

	// 验证命令内容
	assert.Contains(t, output, "go tool pprof -focus=HandleRequest")
	assert.Contains(t, output, "go tool pprof -top")

	// 验证描述
	assert.Contains(t, output, "聚焦到问题函数")
	assert.Contains(t, output, "查看热点函数排名")

	// 验证输出提示
	assert.Contains(t, output, "说明:")
}

// TestPrintSuggestions 测试建议输出
func TestPrintSuggestions(t *testing.T) {
	suggestions := []locator.Suggestion{
		{Category: "immediate", Content: "检查 handler/order.go:123 附近的代码"},
		{Category: "immediate", Content: "使用 pprof 工具进行详细分析"},
		{Category: "long_term", Content: "添加内存监控告警"},
		{Category: "long_term", Content: "定期 review 内存 profile"},
	}

	output := captureOutput(func() {
		printSuggestions(suggestions)
	})

	// 验证建议标题
	assert.Contains(t, output, "建议")

	// 验证分类标签
	assert.Contains(t, output, "[立即]")
	assert.Contains(t, output, "[长期]")

	// 验证建议内容
	assert.Contains(t, output, "检查 handler/order.go:123")
	assert.Contains(t, output, "添加内存监控告警")
}

// TestPrintHotPaths 测试热点路径列表输出
// **Validates: Requirements 7.1**
func TestPrintHotPaths(t *testing.T) {
	hotPaths := []locator.HotPath{
		{
			Chain: locator.CallChain{
				Frames: []locator.StackFrame{
					{
						FunctionName: "main.processRequest",
						ShortName:    "processRequest",
						PackageName:  "main",
						FilePath:     "main.go",
						LineNumber:   50,
						Category:     locator.CategoryBusiness,
					},
				},
				TotalValue:  1000,
				TotalPct:    45.5,
				SampleCount: 10,
			},
			BusinessFrames: []int{0},
			RootCauseIndex: 0,
			ProfileType:    "cpu",
		},
		{
			Chain: locator.CallChain{
				Frames: []locator.StackFrame{
					{
						FunctionName: "runtime.mallocgc",
						ShortName:    "mallocgc",
						PackageName:  "runtime",
						FilePath:     "runtime/malloc.go",
						LineNumber:   1234,
						Category:     locator.CategoryRuntime,
					},
				},
				TotalValue:  500,
				TotalPct:    22.5,
				SampleCount: 5,
			},
			BusinessFrames: []int{},
			RootCauseIndex: -1,
			ProfileType:    "cpu",
		},
	}

	output := captureOutput(func() {
		printHotPaths(hotPaths)
	})

	// 验证热点标题
	assert.Contains(t, output, "热点调用链")

	// 验证热点编号和百分比
	assert.Contains(t, output, "热点 #1")
	assert.Contains(t, output, "45.5%")
	assert.Contains(t, output, "热点 #2")
	assert.Contains(t, output, "22.5%")
}

// TestPrintFindingWithContext 测试带上下文的发现输出
// **Validates: Requirements 7.1, 7.2, 7.6**
func TestPrintFindingWithContext(t *testing.T) {
	finding := rules.Finding{
		RuleID:   "memory_leak",
		RuleName: "Memory Leak Detection",
		Severity: "high",
		Title:    "内存持续增长趋势",
	}

	ctx := &locator.ProblemContext{
		Title:       "内存持续增长趋势",
		Severity:    "high",
		Explanation: "你的程序内存使用量在持续增长。这通常意味着存在内存泄漏。",
		Impact:      "主要消耗点占用 45.2% 的内存分配",
		HotPaths: []locator.HotPath{
			{
				Chain: locator.CallChain{
					Frames: []locator.StackFrame{
						{
							FunctionName: "myapp/handler.HandleOrder",
							ShortName:    "HandleOrder",
							PackageName:  "myapp/handler",
							FilePath:     "handler/order.go",
							LineNumber:   123,
							Category:     locator.CategoryBusiness,
						},
					},
					TotalPct: 45.2,
				},
				BusinessFrames: []int{0},
				RootCauseIndex: 0,
				ProfileType:    "heap",
			},
		},
		Commands: []locator.ExecutableCmd{
			{
				Command:     "go tool pprof -alloc_space ./heap.pprof",
				Description: "查看内存分配热点",
				OutputHint:  "显示累计分配的内存",
			},
		},
		Suggestions: []locator.Suggestion{
			{Category: "immediate", Content: "检查 handler/order.go:123 附近的代码"},
			{Category: "long_term", Content: "添加内存监控告警"},
		},
	}

	output := captureOutput(func() {
		printFindingWithContext(1, finding, ctx)
	})

	// 验证基本信息
	assert.Contains(t, output, "内存持续增长趋势")
	assert.Contains(t, output, "Memory Leak Detection")
	assert.Contains(t, output, "high")

	// 验证问题解释
	assert.Contains(t, output, "问题解释")
	assert.Contains(t, output, "内存泄漏")

	// 验证影响评估
	assert.Contains(t, output, "影响评估")
	assert.Contains(t, output, "45.2%")

	// 验证热点路径
	assert.Contains(t, output, "热点调用链")
	assert.Contains(t, output, "HandleOrder")

	// 验证命令
	assert.Contains(t, output, "调试命令")
	assert.Contains(t, output, "go tool pprof")

	// 注意：建议部分已移除（固定内容，冗余）
}

// TestPrintFindingWithoutContext 测试没有上下文的发现输出（向后兼容）
func TestPrintFindingWithoutContext(t *testing.T) {
	finding := rules.Finding{
		RuleID:   "cpu_hotspot",
		RuleName: "CPU Hotspot Detection",
		Severity: "medium",
		Title:    "CPU 热点检测",
		Evidence: map[string]string{
			"function": "main.processData",
			"cpu_pct":  "35.5%",
		},
		Suggestions: []string{
			"优化算法复杂度",
			"考虑使用缓存",
		},
	}

	output := captureOutput(func() {
		printFindingWithContext(1, finding, nil)
	})

	// 验证基本信息
	assert.Contains(t, output, "CPU 热点检测")
	assert.Contains(t, output, "CPU Hotspot Detection")
	assert.Contains(t, output, "medium")

	// 验证证据（旧格式）
	assert.Contains(t, output, "证据")
	assert.Contains(t, output, "function")
	assert.Contains(t, output, "main.processData")

	// 验证建议（旧格式）
	assert.Contains(t, output, "建议")
	assert.Contains(t, output, "优化算法复杂度")
	assert.Contains(t, output, "考虑使用缓存")
}

// TestPrintWrappedText 测试文本换行
func TestPrintWrappedText(t *testing.T) {
	longText := "这是一段很长的文本，用于测试自动换行功能。它应该在达到指定宽度时自动换行，以保持输出的可读性。"

	output := captureOutput(func() {
		printWrappedText(longText, "   ", 40)
	})

	// 验证输出包含前缀
	assert.True(t, strings.HasPrefix(output, "   "))

	// 验证文本内容存在
	assert.Contains(t, output, "测试")
	assert.Contains(t, output, "换行")
}

// TestPrintWrappedText_WithNewlines 测试带换行符的文本
func TestPrintWrappedText_WithNewlines(t *testing.T) {
	text := "第一段内容。\n\n第二段内容。"

	output := captureOutput(func() {
		printWrappedText(text, "   ", 70)
	})

	// 验证两段都存在
	assert.Contains(t, output, "第一段")
	assert.Contains(t, output, "第二段")
}
