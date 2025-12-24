package rules

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEngine_ValidRules 测试加载有效规则
// **Property 3: Rule Parsing Validity**
// **Validates: Requirements 2.2**
func TestNewEngine_ValidRules(t *testing.T) {
	// 创建临时规则文件
	tempDir, err := os.MkdirTemp("", "rules-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	rulesContent := `rules:
  - id: "test_rule"
    name: "测试规则"
    profile_types: ["heap"]
    condition: "trends.heap_inuse.slope > 10.0"
    actions:
      - type: "report"
        severity: "high"
        title: "测试发现"
        suggestions:
          - "建议1"
          - "建议2"
`
	rulesPath := filepath.Join(tempDir, "rules.yaml")
	err = os.WriteFile(rulesPath, []byte(rulesContent), 0644)
	require.NoError(t, err)

	engine, err := NewEngine(rulesPath)
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.Len(t, engine.rules, 1)
	assert.Equal(t, "test_rule", engine.rules[0].ID)
	assert.Equal(t, "测试规则", engine.rules[0].Name)
}

// TestNewEngine_MissingFile 测试缺失文件
// **Validates: Requirements 2.5**
func TestNewEngine_MissingFile(t *testing.T) {
	engine, err := NewEngine("/nonexistent/rules.yaml")
	assert.Error(t, err)
	assert.Nil(t, engine)
	assert.Contains(t, err.Error(), "not found")
}

// TestNewEngine_EmptyPath 测试空路径
func TestNewEngine_EmptyPath(t *testing.T) {
	engine, err := NewEngine("")
	assert.NoError(t, err)
	assert.Nil(t, engine)
}

// TestNewEngine_InvalidYAML 测试无效 YAML
// **Property 3: Rule Parsing Validity**
// **Validates: Requirements 2.2**
func TestNewEngine_InvalidYAML(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rules-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	rulesPath := filepath.Join(tempDir, "rules.yaml")
	err = os.WriteFile(rulesPath, []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)

	engine, err := NewEngine(rulesPath)
	assert.Error(t, err)
	assert.Nil(t, engine)
}

// TestNewEngine_MissingRequiredFields 测试缺少必需字段
// **Property 3: Rule Parsing Validity**
// **Validates: Requirements 2.2**
func TestNewEngine_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name: "missing id",
			content: `rules:
  - name: "测试"
    profile_types: ["heap"]
    condition: "test"
    actions:
      - type: "report"
`,
			errMsg: "missing id",
		},
		{
			name: "missing name",
			content: `rules:
  - id: "test"
    profile_types: ["heap"]
    condition: "test"
    actions:
      - type: "report"
`,
			errMsg: "missing name",
		},
		{
			name: "missing profile_types",
			content: `rules:
  - id: "test"
    name: "测试"
    condition: "test"
    actions:
      - type: "report"
`,
			errMsg: "missing profile_types",
		},
		{
			name: "missing condition",
			content: `rules:
  - id: "test"
    name: "测试"
    profile_types: ["heap"]
    actions:
      - type: "report"
`,
			errMsg: "missing condition",
		},
		{
			name: "missing actions",
			content: `rules:
  - id: "test"
    name: "测试"
    profile_types: ["heap"]
    condition: "test"
`,
			errMsg: "missing actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "rules-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			rulesPath := filepath.Join(tempDir, "rules.yaml")
			err = os.WriteFile(rulesPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			engine, err := NewEngine(rulesPath)
			assert.Error(t, err)
			assert.Nil(t, engine)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// TestEngine_MatchesProfileType 测试 profile 类型匹配
// **Property 4: Rule Evaluation Consistency**
// **Validates: Requirements 2.6**
func TestEngine_MatchesProfileType(t *testing.T) {
	engine := &Engine{
		rules: []Rule{
			{
				ID:           "heap_rule",
				Name:         "Heap Rule",
				ProfileTypes: []string{"heap"},
				Condition:    "test",
				Actions:      []Action{{Type: "report"}},
			},
			{
				ID:           "multi_rule",
				Name:         "Multi Rule",
				ProfileTypes: []string{"cpu", "heap", "goroutine"},
				Condition:    "test",
				Actions:      []Action{{Type: "report"}},
			},
		},
	}

	// heap_rule 只匹配 heap
	assert.True(t, engine.matchesProfileType(engine.rules[0], "heap"))
	assert.False(t, engine.matchesProfileType(engine.rules[0], "cpu"))
	assert.False(t, engine.matchesProfileType(engine.rules[0], "goroutine"))

	// multi_rule 匹配多种类型
	assert.True(t, engine.matchesProfileType(engine.rules[1], "cpu"))
	assert.True(t, engine.matchesProfileType(engine.rules[1], "heap"))
	assert.True(t, engine.matchesProfileType(engine.rules[1], "goroutine"))
	assert.False(t, engine.matchesProfileType(engine.rules[1], "block"))
}

// TestEngine_Evaluate_NoTrends 测试无趋势数据时的评估
// **Property 4: Rule Evaluation Consistency**
// **Validates: Requirements 2.3, 2.4**
func TestEngine_Evaluate_NoTrends(t *testing.T) {
	engine := &Engine{
		rules: []Rule{
			{
				ID:           "test_rule",
				Name:         "Test Rule",
				ProfileTypes: []string{"heap"},
				Condition:    "trends.heap_inuse.slope > 10.0",
				Actions: []Action{
					{
						Type:     "report",
						Severity: "high",
						Title:    "Test Finding",
					},
				},
			},
		},
	}

	groups := []analyzer.ProfileGroup{
		{Type: "heap", Files: []analyzer.ProfileFile{}},
	}

	// 无趋势数据时不应该产生发现
	findings := engine.Evaluate(groups, nil)
	assert.Empty(t, findings)
}

// TestEngine_Evaluate_WithMatchingTrends 测试有匹配趋势时的评估
// **Property 4: Rule Evaluation Consistency**
// **Validates: Requirements 2.3, 2.4**
func TestEngine_Evaluate_WithMatchingTrends(t *testing.T) {
	engine := &Engine{
		rules: []Rule{
			{
				ID:           "memory_growth",
				Name:         "Memory Growth",
				ProfileTypes: []string{"heap"},
				Condition:    "trends.heap_inuse.slope > 10.0",
				Actions: []Action{
					{
						Type:        "report",
						Severity:    "high",
						Title:       "Memory Growing",
						Suggestions: []string{"Check for leaks"},
						EvidenceTemplate: map[string]string{
							"斜率": "{{.slope}}",
							"R²": "{{.r2}}",
						},
					},
				},
			},
		},
	}

	now := time.Now()
	// 需要至少3个文件才能触发趋势规则
	// 设置时间间隔为1分钟，斜率为 1MB/样本点 = 2MB总变化 / 1分钟 = 2MB/分钟
	groups := []analyzer.ProfileGroup{
		{
			Type: "heap",
			Files: []analyzer.ProfileFile{
				{Path: "/test1.pprof", Time: now},
				{Path: "/test2.pprof", Time: now.Add(30 * time.Second)},
				{Path: "/test3.pprof", Time: now.Add(60 * time.Second)},
			},
		},
	}

	trends := map[string]*analyzer.GroupTrends{
		"heap": {
			HeapInuse: &analyzer.TrendMetrics{
				Slope:     1024 * 1024, // 1MB/样本点，2个间隔 = 2MB总变化，1分钟 = 2MB/分钟
				R2:        0.9,         // > 0.85
				Direction: "increasing",
			},
		},
	}

	findings := engine.Evaluate(groups, trends)
	require.Len(t, findings, 1)
	assert.Equal(t, "memory_growth", findings[0].RuleID)
	assert.Equal(t, "Memory Growth", findings[0].RuleName)
	assert.Equal(t, "high", findings[0].Severity)
	assert.Equal(t, "Memory Growing", findings[0].Title)
	assert.Contains(t, findings[0].Suggestions, "Check for leaks")

	// 验证证据模板变量被正确替换
	// 斜率 = 1MB/样本点 * 2个间隔 / 1分钟 = 2MB/分钟
	assert.Equal(t, "2.00 MB", findings[0].Evidence["斜率"])
	assert.Equal(t, "0.90", findings[0].Evidence["R²"])
}

// TestEngine_Evaluate_NilEngine 测试 nil 引擎
func TestEngine_Evaluate_NilEngine(t *testing.T) {
	var engine *Engine
	findings := engine.Evaluate(nil, nil)
	assert.Nil(t, findings)
}

// TestEngine_Evaluate_EmptyRules 测试空规则
func TestEngine_Evaluate_EmptyRules(t *testing.T) {
	engine := &Engine{rules: []Rule{}}
	findings := engine.Evaluate(nil, nil)
	assert.Nil(t, findings)
}

// TestLoadDefaultRules 测试加载默认规则文件
func TestLoadDefaultRules(t *testing.T) {
	// 检查默认规则文件是否存在
	if _, err := os.Stat("../../assets/default_rules.yaml"); os.IsNotExist(err) {
		t.Skip("默认规则文件不存在，跳过测试")
	}

	engine, err := NewEngine("../../assets/default_rules.yaml")
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.True(t, len(engine.rules) > 0, "应该加载至少一条规则")
}

// TestEngine_BuildEvidence_TemplateReplacement 测试证据模板变量替换
func TestEngine_BuildEvidence_TemplateReplacement(t *testing.T) {
	engine := &Engine{}

	template := map[string]string{
		"内存增长速率": "{{.slope}}/分钟",
		"线性相关度":  "{{.r2}} (1.0为完美线性)",
		"时间范围":   "{{.duration}}",
		"文件数量":   "{{.file_count}} 个文件",
	}

	trends := &analyzer.GroupTrends{
		HeapInuse: &analyzer.TrendMetrics{
			Slope:     5 * 1024 * 1024, // 5MB/样本点
			R2:        0.95,
			Direction: "increasing",
		},
	}

	now := time.Now()
	group := analyzer.ProfileGroup{
		Type: "heap",
		Files: []analyzer.ProfileFile{
			{Path: "/test1.pprof", Time: now},
			{Path: "/test2.pprof", Time: now.Add(30 * time.Second)},
			{Path: "/test3.pprof", Time: now.Add(60 * time.Second)},
		},
	}

	evidence := engine.buildEvidence(template, trends, group)

	// 斜率 = 5MB/样本点 * 2个间隔 / 1分钟 = 10MB/分钟
	assert.Equal(t, "10.00 MB/分钟", evidence["内存增长速率"])
	assert.Equal(t, "0.95 (1.0为完美线性)", evidence["线性相关度"])
	assert.Equal(t, "3 个文件", evidence["文件数量"])
	assert.Equal(t, "1.0 分钟", evidence["时间范围"])
}

// TestEngine_BuildEvidence_NilInputs 测试空输入
func TestEngine_BuildEvidence_NilInputs(t *testing.T) {
	engine := &Engine{}

	// nil template
	evidence := engine.buildEvidence(nil, &analyzer.GroupTrends{}, analyzer.ProfileGroup{})
	assert.Nil(t, evidence)

	// nil trends
	evidence = engine.buildEvidence(map[string]string{"key": "value"}, nil, analyzer.ProfileGroup{})
	assert.Nil(t, evidence)
}
