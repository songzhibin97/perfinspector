package reporter

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/songzhibin97/perfinspector/pkg/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateHTMLReport_Basic 测试基本 HTML 报告生成
// **Property 1: HTML Report Content Completeness**
// **Validates: Requirements 1.2, 1.6, 1.7**
func TestGenerateHTMLReport_Basic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type: "cpu",
			Files: []analyzer.ProfileFile{
				{
					Path: "/path/to/cpu1.pprof",
					Time: time.Date(2023, 11, 15, 14, 30, 0, 0, time.UTC),
					Size: 1024,
				},
				{
					Path: "/path/to/cpu2.pprof",
					Time: time.Date(2023, 11, 15, 14, 35, 0, 0, time.UTC),
					Size: 2048,
				},
			},
		},
	}

	err = GenerateHTMLReport(groups, nil, nil, outputPath)
	require.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// 读取并验证内容
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// 验证是自包含的 HTML
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "<style>")
	assert.Contains(t, html, "</style>")

	// 验证包含文件信息
	assert.Contains(t, html, "cpu1.pprof")
	assert.Contains(t, html, "cpu2.pprof")

	// 验证时间戳格式 (RFC3339)
	assert.Contains(t, html, "2023-11-15T14:30:00Z")
	assert.Contains(t, html, "2023-11-15T14:35:00Z")

	// 验证文件大小
	assert.Contains(t, html, "1.00 KB")
	assert.Contains(t, html, "2.00 KB")
}

// TestGenerateHTMLReport_WithTimeRange 测试包含时间范围的报告
// **Property 1: HTML Report Content Completeness**
// **Validates: Requirements 1.3**
func TestGenerateHTMLReport_WithTimeRange(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type: "heap",
			Files: []analyzer.ProfileFile{
				{
					Path: "/path/to/heap1.pprof",
					Time: time.Date(2023, 11, 15, 10, 0, 0, 0, time.UTC),
					Size: 1024,
				},
				{
					Path: "/path/to/heap2.pprof",
					Time: time.Date(2023, 11, 15, 11, 0, 0, 0, time.UTC),
					Size: 2048,
				},
			},
		},
	}

	err = GenerateHTMLReport(groups, nil, nil, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// 验证时间范围显示
	assert.Contains(t, html, "时间范围")
	assert.Contains(t, html, "持续时间")
}

// TestGenerateHTMLReport_WithFindings 测试包含规则发现的报告
// **Property 5: Findings in Report Output**
// **Validates: Requirements 2.7**
func TestGenerateHTMLReport_WithFindings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type:  "heap",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	findings := []rules.Finding{
		{
			RuleID:      "test_rule",
			RuleName:    "Test Rule",
			Severity:    "high",
			Title:       "Test Finding Title",
			Suggestions: []string{"Suggestion 1", "Suggestion 2"},
		},
	}

	err = GenerateHTMLReport(groups, nil, findings, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// 验证发现信息（注意：suggestions 已从 HTML 报告中移除以保持简洁）
	assert.Contains(t, html, "Test Finding Title")
	assert.Contains(t, html, "Test Rule")
	assert.Contains(t, html, "test_rule")
	assert.Contains(t, html, "high")
}

// TestGenerateHTMLReport_WithTrends 测试包含趋势的报告
// **Property 8: Trend Display Filtering**
// **Validates: Requirements 4.4, 4.5**
func TestGenerateHTMLReport_WithTrends(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type: "heap",
			Files: []analyzer.ProfileFile{
				{Path: "/test1.pprof", Time: time.Now(), Size: 100},
				{Path: "/test2.pprof", Time: time.Now().Add(time.Hour), Size: 200},
			},
		},
	}

	trends := map[string]*analyzer.GroupTrends{
		"heap": {
			HeapInuse: &analyzer.TrendMetrics{
				Slope:     15.5,
				R2:        0.95, // > 0.7, 应该显示
				Direction: "increasing",
			},
		},
	}

	err = GenerateHTMLReport(groups, trends, nil, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// 验证趋势信息（R² > 0.7 应该显示）
	assert.Contains(t, html, "趋势分析")
	assert.Contains(t, html, "increasing")
}

// TestGenerateHTMLReport_TrendsFiltering 测试趋势过滤（R² <= 0.7 不显示）
// **Property 8: Trend Display Filtering**
// **Validates: Requirements 4.5**
func TestGenerateHTMLReport_TrendsFiltering(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type: "heap",
			Files: []analyzer.ProfileFile{
				{Path: "/test1.pprof", Time: time.Now(), Size: 100},
			},
		},
	}

	trends := map[string]*analyzer.GroupTrends{
		"heap": {
			HeapInuse: &analyzer.TrendMetrics{
				Slope:     15.5,
				R2:        0.5, // <= 0.7, 不应该显示趋势区块
				Direction: "increasing",
			},
		},
	}

	err = GenerateHTMLReport(groups, trends, nil, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// R² <= 0.7 时不应该显示趋势分析区块
	// 注意：HTML 中可能有 "趋势分析" 的标题模板，但 HasTrends 应该是 false
	// 检查是否没有显示具体的趋势数据
	assert.False(t, strings.Contains(html, "R²=0.50"), "低 R² 的趋势不应该显示")
}

// TestGenerateHTMLReport_InvalidPath 测试无效输出路径
// **Property 2: HTML Report File Output**
// **Validates: Requirements 1.5**
func TestGenerateHTMLReport_InvalidPath(t *testing.T) {
	groups := []analyzer.ProfileGroup{
		{
			Type:  "cpu",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	// 尝试写入不存在的目录
	err := GenerateHTMLReport(groups, nil, nil, "/nonexistent/dir/report.html")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create output file")
}

// TestGenerateHTMLReport_EmptyGroups 测试空分组
func TestGenerateHTMLReport_EmptyGroups(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	err = GenerateHTMLReport([]analyzer.ProfileGroup{}, nil, nil, outputPath)
	require.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(outputPath)
	require.NoError(t, err)
}

// TestGenerateHTMLReport_MultipleGroups 测试多个分组
// **Property 1: HTML Report Content Completeness**
// **Validates: Requirements 1.2**
func TestGenerateHTMLReport_MultipleGroups(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type: "cpu",
			Files: []analyzer.ProfileFile{
				{Path: "/cpu.pprof", Time: time.Now(), Size: 1024},
			},
		},
		{
			Type: "heap",
			Files: []analyzer.ProfileFile{
				{Path: "/heap.pprof", Time: time.Now(), Size: 2048},
			},
		},
		{
			Type: "goroutine",
			Files: []analyzer.ProfileFile{
				{Path: "/goroutine.pprof", Time: time.Now(), Size: 512},
			},
		},
	}

	err = GenerateHTMLReport(groups, nil, nil, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// 验证所有分组都包含在报告中
	assert.Contains(t, html, "cpu")
	assert.Contains(t, html, "heap")
	assert.Contains(t, html, "goroutine")
	assert.Contains(t, html, "cpu.pprof")
	assert.Contains(t, html, "heap.pprof")
	assert.Contains(t, html, "goroutine.pprof")
}

// Feature: problem-locator, Property 8: Report Category Markers
// Validates: Requirements 7.2, 7.4, 7.5

// TestHTMLReport_Property_CategoryMarkers tests that all stack frames have correct category markers
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.2, 7.4, 7.5**
func TestHTMLReport_Property_CategoryMarkers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type:  "cpu",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	findings := []rules.Finding{
		{
			RuleID:   "test_rule",
			RuleName: "Test Rule",
			Severity: "high",
			Title:    "Test Finding",
		},
	}

	// Create ProblemContext with various category frames
	contexts := map[string]*locator.ProblemContext{
		"test_rule": {
			Title:       "Test Problem",
			Severity:    "high",
			Explanation: "Test explanation",
			Impact:      "Test impact",
			HotPaths: []locator.HotPath{
				{
					Chain: locator.CallChain{
						Frames: []locator.StackFrame{
							{
								FunctionName: "main.main",
								ShortName:    "main",
								PackageName:  "main",
								FilePath:     "/app/main.go",
								LineNumber:   10,
								Category:     locator.CategoryBusiness,
							},
							{
								FunctionName: "net/http.ListenAndServe",
								ShortName:    "ListenAndServe",
								PackageName:  "net/http",
								FilePath:     "/usr/local/go/src/net/http/server.go",
								LineNumber:   100,
								Category:     locator.CategoryStdlib,
							},
							{
								FunctionName: "runtime.goexit",
								ShortName:    "goexit",
								PackageName:  "runtime",
								FilePath:     "/usr/local/go/src/runtime/asm_amd64.s",
								LineNumber:   200,
								Category:     locator.CategoryRuntime,
							},
							{
								FunctionName: "github.com/gin-gonic/gin.(*Engine).Run",
								ShortName:    "Run",
								PackageName:  "github.com/gin-gonic/gin",
								FilePath:     "/go/pkg/mod/github.com/gin-gonic/gin@v1.9.0/gin.go",
								LineNumber:   50,
								Category:     locator.CategoryThirdParty,
							},
						},
						TotalPct:    45.5,
						SampleCount: 100,
					},
					BusinessFrames: []int{0},
					RootCauseIndex: 0,
					ProfileType:    "cpu",
				},
			},
			Commands: []locator.ExecutableCmd{
				{
					Command:     "go tool pprof -focus=main /test.pprof",
					Description: "Focus on main function",
					OutputHint:  "Shows call graph focused on main",
				},
			},
			Suggestions: []locator.Suggestion{
				{Category: "immediate", Content: "Check main.go:10"},
				{Category: "long_term", Content: "Add monitoring"},
			},
		},
	}

	err = GenerateHTMLReportWithContext(groups, nil, findings, contexts, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// Property 8: For any StackFrame rendered in HTML report:
	// 1. Business frames SHALL have distinct visual marker (icon/color/class)
	assert.Contains(t, html, "frame-business", "Business frames should have frame-business class")

	// 2. The marker SHALL correctly correspond to the frame's Category
	assert.Contains(t, html, "frame-runtime", "Runtime frames should have frame-runtime class")
	assert.Contains(t, html, "frame-stdlib", "Stdlib frames should have frame-stdlib class")
	assert.Contains(t, html, "frame-third-party", "Third-party frames should have frame-third-party class")

	// 3. File:line links in HTML SHALL use file:// protocol for local paths
	assert.Contains(t, html, "file:///app/main.go#L10", "Business frame should have file:// link with line number")
	assert.Contains(t, html, "file:///usr/local/go/src/net/http/server.go#L100", "Stdlib frame should have file:// link")
	assert.Contains(t, html, "file:///usr/local/go/src/runtime/asm_amd64.s#L200", "Runtime frame should have file:// link")

	// Verify category icons are present
	assert.Contains(t, html, "💼", "Business category icon should be present")
	assert.Contains(t, html, "📚", "Stdlib category icon should be present")
	assert.Contains(t, html, "⚙️", "Runtime category icon should be present")
	assert.Contains(t, html, "📦", "Third-party category icon should be present")

	// Verify business frame highlighting
	assert.Contains(t, html, "highlight", "Business frames should be highlighted")
	assert.Contains(t, html, "根因", "Root cause tag should be present")
}

// TestHTMLReport_Property_CategoryMarkersConsistency tests that category markers are consistent
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.2, 7.4**
func TestHTMLReport_Property_CategoryMarkersConsistency(t *testing.T) {
	// Test that GetCategoryClass returns correct CSS class for each category
	testCases := []struct {
		category      locator.CodeCategory
		expectedClass string
	}{
		{locator.CategoryRuntime, "frame-runtime"},
		{locator.CategoryStdlib, "frame-stdlib"},
		{locator.CategoryThirdParty, "frame-third-party"},
		{locator.CategoryBusiness, "frame-business"},
		{locator.CategoryUnknown, "frame-unknown"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.category), func(t *testing.T) {
			class := GetCategoryClass(tc.category)
			assert.Equal(t, tc.expectedClass, class, "Category %s should map to class %s", tc.category, tc.expectedClass)
		})
	}
}

// TestHTMLReport_Property_FileLinkGeneration tests file:// link generation
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.4, 7.5**
func TestHTMLReport_Property_FileLinkGeneration(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		lineNumber   int64
		expectedLink string
	}{
		{
			name:         "absolute path with line number",
			filePath:     "/app/main.go",
			lineNumber:   42,
			expectedLink: "file:///app/main.go#L42",
		},
		{
			name:         "absolute path without line number",
			filePath:     "/app/main.go",
			lineNumber:   0,
			expectedLink: "file:///app/main.go",
		},
		{
			name:         "unknown file path",
			filePath:     "unknown",
			lineNumber:   10,
			expectedLink: "",
		},
		{
			name:         "empty file path",
			filePath:     "",
			lineNumber:   10,
			expectedLink: "",
		},
		{
			name:         "relative path (no link)",
			filePath:     "main.go",
			lineNumber:   10,
			expectedLink: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			link := generateFileLink(tc.filePath, tc.lineNumber)
			assert.Equal(t, tc.expectedLink, link)
		})
	}
}

// TestHTMLReport_Property_HotPathConversion tests hot path conversion for HTML
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.2, 7.4, 7.5**
func TestHTMLReport_Property_HotPathConversion(t *testing.T) {
	hotPaths := []locator.HotPath{
		{
			Chain: locator.CallChain{
				Frames: []locator.StackFrame{
					{
						FunctionName: "main.handler",
						ShortName:    "handler",
						PackageName:  "main",
						FilePath:     "/app/handler.go",
						LineNumber:   25,
						Category:     locator.CategoryBusiness,
					},
					{
						FunctionName: "fmt.Println",
						ShortName:    "Println",
						PackageName:  "fmt",
						FilePath:     "/usr/local/go/src/fmt/print.go",
						LineNumber:   100,
						Category:     locator.CategoryStdlib,
					},
				},
				TotalPct:    30.5,
				SampleCount: 50,
			},
			BusinessFrames: []int{0},
			RootCauseIndex: 0,
			ProfileType:    "cpu",
		},
	}

	htmlHotPaths := ConvertHotPathsForHTML(hotPaths)

	require.Len(t, htmlHotPaths, 1)
	hp := htmlHotPaths[0]

	// Verify index
	assert.Equal(t, 1, hp.Index)

	// Verify percentage
	assert.Equal(t, 30.5, hp.TotalPct)

	// Verify frames
	require.Len(t, hp.Frames, 2)

	// First frame (business)
	assert.Equal(t, "business", hp.Frames[0].Category)
	assert.Equal(t, "💼", hp.Frames[0].CategoryIcon)
	assert.Equal(t, "handler", hp.Frames[0].ShortName)
	assert.Equal(t, template.URL("file:///app/handler.go#L25"), hp.Frames[0].FileLink)
	assert.True(t, hp.Frames[0].IsHighlight)
	assert.Equal(t, "根因", hp.Frames[0].HighlightTag)
	assert.False(t, hp.Frames[0].IsNewSection) // First frame

	// Second frame (stdlib)
	assert.Equal(t, "stdlib", hp.Frames[1].Category)
	assert.Equal(t, "📚", hp.Frames[1].CategoryIcon)
	assert.Equal(t, "Println", hp.Frames[1].ShortName)
	assert.False(t, hp.Frames[1].IsHighlight)
	assert.True(t, hp.Frames[1].IsNewSection) // Category changed
}

// TestHTMLReport_Property_CommandConversion tests command conversion for HTML
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.6**
func TestHTMLReport_Property_CommandConversion(t *testing.T) {
	commands := []locator.ExecutableCmd{
		{
			Command:     "go tool pprof -focus=main /test.pprof",
			Description: "Focus on main",
			OutputHint:  "Shows main function",
		},
		{
			Command:     "go tool pprof -top /test.pprof",
			Description: "Show top functions",
			OutputHint:  "Lists top consumers",
		},
	}

	htmlCommands := ConvertCommandsForHTML(commands)

	require.Len(t, htmlCommands, 2)

	assert.Equal(t, 1, htmlCommands[0].Index)
	assert.Equal(t, "go tool pprof -focus=main /test.pprof", htmlCommands[0].Command)
	assert.Equal(t, "Focus on main", htmlCommands[0].Description)
	assert.Equal(t, "Shows main function", htmlCommands[0].OutputHint)

	assert.Equal(t, 2, htmlCommands[1].Index)
}

// TestHTMLReport_Property_SuggestionConversion tests suggestion conversion for HTML
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.6**
func TestHTMLReport_Property_SuggestionConversion(t *testing.T) {
	suggestions := []locator.Suggestion{
		{Category: "immediate", Content: "Fix this now"},
		{Category: "long_term", Content: "Plan for later"},
		{Category: "immediate", Content: "Also fix this"},
	}

	immediate, longTerm := ConvertSuggestionsForHTML(suggestions)

	require.Len(t, immediate, 2)
	require.Len(t, longTerm, 1)

	assert.Equal(t, "Fix this now", immediate[0].Content)
	assert.Equal(t, "Also fix this", immediate[1].Content)
	assert.Equal(t, "Plan for later", longTerm[0].Content)
}

// TestHTMLReport_WithProblemContext_NoBusinessFrames tests report with no business frames
// **Property 8: Report Category Markers**
// **Validates: Requirements 7.2, 7.5**
func TestHTMLReport_WithProblemContext_NoBusinessFrames(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type:  "cpu",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	findings := []rules.Finding{
		{
			RuleID:   "gc_rule",
			RuleName: "GC Rule",
			Severity: "medium",
			Title:    "GC Issue",
		},
	}

	// Create ProblemContext with only runtime frames (no business code)
	contexts := map[string]*locator.ProblemContext{
		"gc_rule": {
			Title:       "GC Problem",
			Severity:    "medium",
			Explanation: "GC is consuming resources",
			Impact:      "High GC overhead",
			HotPaths: []locator.HotPath{
				{
					Chain: locator.CallChain{
						Frames: []locator.StackFrame{
							{
								FunctionName: "runtime.mallocgc",
								ShortName:    "mallocgc",
								PackageName:  "runtime",
								FilePath:     "/usr/local/go/src/runtime/malloc.go",
								LineNumber:   100,
								Category:     locator.CategoryRuntime,
							},
							{
								FunctionName: "runtime.gcBgMarkWorker",
								ShortName:    "gcBgMarkWorker",
								PackageName:  "runtime",
								FilePath:     "/usr/local/go/src/runtime/mgc.go",
								LineNumber:   200,
								Category:     locator.CategoryRuntime,
							},
						},
						TotalPct:    60.0,
						SampleCount: 200,
					},
					BusinessFrames: []int{}, // No business frames
					RootCauseIndex: -1,      // No root cause
					ProfileType:    "cpu",
				},
			},
		},
	}

	err = GenerateHTMLReportWithContext(groups, nil, findings, contexts, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// Should show warning about no business code
	assert.Contains(t, html, "没有业务代码", "Should show warning when no business frames")
}

// TestConvertSuggestionsForHTML tests the suggestion conversion
func TestConvertSuggestionsForHTML(t *testing.T) {
	suggestions := []locator.Suggestion{
		{
			Category: "immediate",
			Content:  "Fix this now",
		},
		{
			Category: "long_term",
			Content:  "Plan for later",
		},
		{
			Category: "immediate",
			Content:  "Another immediate fix",
		},
	}

	immediate, longTerm := ConvertSuggestionsForHTML(suggestions)

	// Verify immediate suggestions
	require.Len(t, immediate, 2)
	assert.Equal(t, "Fix this now", immediate[0].Content)
	assert.Equal(t, "Another immediate fix", immediate[1].Content)

	// Verify long term suggestions
	require.Len(t, longTerm, 1)
	assert.Equal(t, "Plan for later", longTerm[0].Content)
}

// TestHTMLReport_WithSuggestions tests report generation with suggestions
func TestHTMLReport_WithSuggestions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type:  "cpu",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	findings := []rules.Finding{
		{
			RuleID:   "test_rule",
			RuleName: "Test Rule",
			Severity: "medium",
			Title:    "Test Finding",
		},
	}

	// Create ProblemContext with suggestions
	contexts := map[string]*locator.ProblemContext{
		"test_rule": {
			Title:       "Test Problem",
			Severity:    "medium",
			Explanation: "Test explanation",
			Suggestions: []locator.Suggestion{
				{Category: "immediate", Content: "Do something"},
			},
		},
	}

	err = GenerateHTMLReportWithContext(groups, nil, findings, contexts, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// Should have suggestions section
	assert.Contains(t, html, `suggestions-section`, "Should have suggestions section")
}

// TestHTMLReport_WithBusinessCode_NoWarning tests that warning is not shown when business code exists
// **Property 4: 包含 Business 帧的 HotPath 不显示警告**
// **Validates: Requirements 1.4**
func TestHTMLReport_WithBusinessCode_NoWarning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "html-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "report.html")

	groups := []analyzer.ProfileGroup{
		{
			Type:  "cpu",
			Files: []analyzer.ProfileFile{{Path: "/test.pprof", Time: time.Now(), Size: 100}},
		},
	}

	findings := []rules.Finding{
		{
			RuleID:   "test_rule",
			RuleName: "Test Rule",
			Severity: "high",
			Title:    "Test Finding",
		},
	}

	// Create ProblemContext WITH business frames
	contexts := map[string]*locator.ProblemContext{
		"test_rule": {
			Title:       "Test Problem",
			Severity:    "high",
			Explanation: "Test explanation",
			Impact:      "Test impact",
			HotPaths: []locator.HotPath{
				{
					Chain: locator.CallChain{
						Frames: []locator.StackFrame{
							{
								FunctionName: "main.handler",
								ShortName:    "handler",
								PackageName:  "main",
								FilePath:     "/app/handler.go",
								LineNumber:   25,
								Category:     locator.CategoryBusiness, // Business code!
							},
							{
								FunctionName: "runtime.goexit",
								ShortName:    "goexit",
								PackageName:  "runtime",
								FilePath:     "/usr/local/go/src/runtime/asm_amd64.s",
								LineNumber:   200,
								Category:     locator.CategoryRuntime,
							},
						},
						TotalPct:    45.5,
						SampleCount: 100,
					},
					BusinessFrames: []int{0}, // Has business frame at index 0
					RootCauseIndex: 0,
					ProfileType:    "cpu",
				},
			},
		},
	}

	err = GenerateHTMLReportWithContext(groups, nil, findings, contexts, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	html := string(content)

	// Should NOT show warning when business code exists
	assert.NotContains(t, html, "该路径中没有业务代码", "Should NOT show warning when business code exists")
	assert.NotContains(t, html, "运行时/GC 问题", "Should NOT show runtime/GC warning when business code exists")

	// Should show business frame
	assert.Contains(t, html, "frame-business", "Should show business frame")
	assert.Contains(t, html, "handler", "Should show business function name")
}
