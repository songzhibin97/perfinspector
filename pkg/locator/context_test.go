package locator

import (
	"strings"
	"testing"
	"testing/quick"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/rules"
	"github.com/stretchr/testify/assert"
)

// Feature: problem-locator, Property 6: Problem Context Completeness
// Validates: Requirements 5.1, 5.3, 5.4, 5.5

// createTestFinding creates a test Finding with the given parameters
func createTestFinding(title, severity string, suggestions []string) rules.Finding {
	return rules.Finding{
		RuleID:      "test-rule",
		RuleName:    "Test Rule",
		Severity:    severity,
		Title:       title,
		Evidence:    map[string]string{"key": "value"},
		Suggestions: suggestions,
	}
}

// createTestProfileWithSamples creates a test profile with samples
func createTestProfileWithSamples(funcNames []string, value int64) *profile.Profile {
	if len(funcNames) == 0 {
		return &profile.Profile{Sample: []*profile.Sample{}}
	}

	locations := make([]*profile.Location, len(funcNames))
	// pprof stores locations from leaf to root
	for i, name := range funcNames {
		fn := &profile.Function{
			ID:       uint64(i + 1),
			Name:     name,
			Filename: name + ".go",
		}
		locations[len(funcNames)-1-i] = &profile.Location{
			ID:   uint64(i + 1),
			Line: []profile.Line{{Function: fn, Line: int64(i + 1)}},
		}
	}

	return &profile.Profile{
		Sample: []*profile.Sample{
			{
				Location: locations,
				Value:    []int64{value},
			},
		},
	}
}

// TestNewContextGenerator tests the constructor
func TestNewContextGenerator(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	generator := NewContextGenerator(analyzer)

	assert.NotNil(t, generator)
	assert.Equal(t, analyzer, generator.analyzer)
}

// TestGenerateContext_Basic tests basic context generation
func TestGenerateContext_Basic(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)
	generator := NewContextGenerator(analyzer)

	finding := createTestFinding("CPU 热点问题", "high", []string{"优化算法"})
	funcNames := []string{
		"github.com/myapp/handler.ProcessRequest",
		"encoding/json.Marshal",
		"runtime.mallocgc",
	}
	p := createTestProfileWithSamples(funcNames, 1000)
	profiles := map[string]*profile.Profile{"cpu": p}

	ctx := generator.GenerateContext(finding, profiles)

	assert.NotNil(t, ctx)
	assert.Equal(t, "CPU 热点问题", ctx.Title)
	assert.Equal(t, "high", ctx.Severity)
	assert.NotEmpty(t, ctx.Explanation)
	assert.NotEmpty(t, ctx.Impact)
	assert.True(t, len(ctx.HotPaths) > 0)
	assert.True(t, len(ctx.Commands) > 0)
	assert.True(t, len(ctx.Suggestions) > 0)
}

// TestGenerateContext_NilAnalyzer tests context generation with nil analyzer
func TestGenerateContext_NilAnalyzer(t *testing.T) {
	generator := &ContextGenerator{analyzer: nil}

	finding := createTestFinding("Test", "high", nil)
	profiles := map[string]*profile.Profile{}

	ctx := generator.GenerateContext(finding, profiles)

	assert.Nil(t, ctx)
}

// TestGenerateContext_EmptyProfile tests context generation with empty profile
func TestGenerateContext_EmptyProfile(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)
	generator := NewContextGenerator(analyzer)

	finding := createTestFinding("内存问题", "medium", []string{"检查内存"})
	profiles := map[string]*profile.Profile{}

	ctx := generator.GenerateContext(finding, profiles)

	assert.NotNil(t, ctx)
	assert.Equal(t, "内存问题", ctx.Title)
	assert.Equal(t, "medium", ctx.Severity)
	assert.NotEmpty(t, ctx.Explanation)
}

// TestDetermineProfileType tests profile type detection
func TestDetermineProfileType(t *testing.T) {
	tests := []struct {
		name     string
		finding  rules.Finding
		expected string
	}{
		{
			name:     "cpu in title",
			finding:  createTestFinding("CPU 热点", "high", nil),
			expected: "cpu",
		},
		{
			name:     "memory in title",
			finding:  createTestFinding("内存泄漏", "high", nil),
			expected: "heap",
		},
		{
			name:     "heap in title",
			finding:  createTestFinding("Heap 增长", "high", nil),
			expected: "heap",
		},
		{
			name:     "goroutine in title",
			finding:  createTestFinding("Goroutine 泄漏", "high", nil),
			expected: "goroutine",
		},
		{
			name:     "协程 in title",
			finding:  createTestFinding("协程数量增长", "high", nil),
			expected: "goroutine",
		},
		{
			name:     "default to cpu",
			finding:  createTestFinding("性能问题", "high", nil),
			expected: "cpu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineProfileType(tt.finding)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeSeverity tests severity normalization
func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"CRITICAL", "critical"},
		{"严重", "critical"},
		{"high", "high"},
		{"HIGH", "high"},
		{"高", "high"},
		{"medium", "medium"},
		{"MEDIUM", "medium"},
		{"中", "medium"},
		{"low", "low"},
		{"LOW", "low"},
		{"低", "low"},
		{"unknown", "medium"},
		{"", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSeverity(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateExplanation tests explanation generation
func TestGenerateExplanation(t *testing.T) {
	t.Run("with hot paths and root cause", func(t *testing.T) {
		finding := createTestFinding("内存泄漏", "high", nil)
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "main.main", ShortName: "main", FilePath: "main.go", LineNumber: 10, Category: CategoryBusiness},
						{FunctionName: "runtime.mallocgc", ShortName: "mallocgc", FilePath: "malloc.go", LineNumber: 100, Category: CategoryRuntime},
					},
				},
				RootCauseIndex: 0,
			},
		}

		explanation := GenerateExplanation(finding, hotPaths)

		assert.NotEmpty(t, explanation)
		assert.Contains(t, explanation, "内存")
		assert.Contains(t, explanation, "main")
	})

	t.Run("without business code", func(t *testing.T) {
		finding := createTestFinding("CPU 问题", "high", nil)
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "runtime.mallocgc", ShortName: "mallocgc", Category: CategoryRuntime},
					},
				},
				RootCauseIndex: -1,
			},
		}

		explanation := GenerateExplanation(finding, hotPaths)

		assert.NotEmpty(t, explanation)
		assert.Contains(t, explanation, "没有业务代码")
	})

	t.Run("empty hot paths", func(t *testing.T) {
		finding := createTestFinding("问题", "high", nil)

		explanation := GenerateExplanation(finding, nil)

		assert.NotEmpty(t, explanation)
	})
}

// TestGenerateImpact tests impact generation
func TestGenerateImpact(t *testing.T) {
	t.Run("with hot paths", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					TotalPct: 45.5,
					Frames: []StackFrame{
						{FunctionName: "main.main", ShortName: "main", FilePath: "main.go", LineNumber: 10, Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
				ProfileType:    "cpu",
			},
		}

		impact := GenerateImpact(hotPaths, "cpu")

		assert.NotEmpty(t, impact)
		assert.Contains(t, impact, "45.5%")
		assert.Contains(t, impact, "CPU")
	})

	t.Run("empty hot paths", func(t *testing.T) {
		impact := GenerateImpact(nil, "cpu")

		assert.Contains(t, impact, "无法评估")
	})

	t.Run("heap profile type", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					TotalPct: 30.0,
				},
				RootCauseIndex: -1,
				ProfileType:    "heap",
			},
		}

		impact := GenerateImpact(hotPaths, "heap")

		assert.Contains(t, impact, "内存")
	})

	t.Run("goroutine profile type", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					TotalPct: 20.0,
				},
				RootCauseIndex: -1,
				ProfileType:    "goroutine",
			},
		}

		impact := GenerateImpact(hotPaths, "goroutine")

		assert.Contains(t, impact, "goroutine")
	})
}

// TestGenerateSuggestions tests suggestion generation
func TestGenerateSuggestions(t *testing.T) {
	t.Run("with finding suggestions", func(t *testing.T) {
		finding := createTestFinding("问题", "high", []string{"建议1", "建议2"})

		suggestions := GenerateSuggestions(finding, nil)

		assert.True(t, len(suggestions) >= 2)
		assert.Equal(t, "immediate", suggestions[0].Category)
		assert.Equal(t, "建议1", suggestions[0].Content)
	})

	t.Run("with hot paths", func(t *testing.T) {
		finding := createTestFinding("问题", "high", nil)
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "main.main", ShortName: "main", FilePath: "main.go", LineNumber: 10, Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
				ProfileType:    "cpu",
			},
		}

		suggestions := GenerateSuggestions(finding, hotPaths)

		assert.True(t, len(suggestions) > 0)
		// Should have location-specific suggestion
		hasLocationSuggestion := false
		for _, s := range suggestions {
			if strings.Contains(s.Content, "main.go:10") {
				hasLocationSuggestion = true
				break
			}
		}
		assert.True(t, hasLocationSuggestion)
	})

	t.Run("empty inputs", func(t *testing.T) {
		finding := createTestFinding("问题", "high", nil)

		suggestions := GenerateSuggestions(finding, nil)

		// Should have at least one default suggestion
		assert.True(t, len(suggestions) > 0)
	})
}

// TestGenerateCommands tests command generation
func TestGenerateCommands(t *testing.T) {
	t.Run("basic commands", func(t *testing.T) {
		commands := generateCommands("cpu", nil, nil)

		assert.True(t, len(commands) >= 2) // top and web commands
	})

	t.Run("with hot paths", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "main.main", ShortName: "main", Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
			},
		}

		commands := generateCommands("cpu", hotPaths, nil)

		assert.True(t, len(commands) >= 3) // top, focus, list, web commands
	})

	t.Run("with profile paths", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "main.main", ShortName: "main", Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
			},
		}
		profilePaths := []string{"./testdata/heap1.pprof", "./testdata/heap2.pprof"}

		commands := generateCommands("heap", hotPaths, profilePaths)

		// Should have commands with actual paths
		hasActualPath := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "testdata/heap1.pprof") {
				hasActualPath = true
				break
			}
		}
		assert.True(t, hasActualPath, "Commands should use actual profile paths")
	})
}

// TestGenerateFocusCommand tests focus command generation
func TestGenerateFocusCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateFocusCommand("profile.pprof", "HandleRequest")

	assert.Contains(t, cmd.Command, "go tool pprof")
	assert.Contains(t, cmd.Command, "-focus=HandleRequest")
	assert.Contains(t, cmd.Command, "profile.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestGenerateTopCommand tests top command generation
func TestGenerateTopCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateTopCommand("profile.pprof")

	assert.Contains(t, cmd.Command, "go tool pprof")
	assert.Contains(t, cmd.Command, "-top")
	assert.Contains(t, cmd.Command, "profile.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestGenerateListCommand tests list command generation
func TestGenerateListCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateListCommand("profile.pprof", "HandleRequest")

	assert.Contains(t, cmd.Command, "go tool pprof")
	assert.Contains(t, cmd.Command, "-list=HandleRequest")
	assert.Contains(t, cmd.Command, "profile.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestGenerateWebCommand tests web command generation
func TestGenerateWebCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateWebCommand("profile.pprof")

	assert.Contains(t, cmd.Command, "go tool pprof")
	assert.Contains(t, cmd.Command, "-http=")
	assert.Contains(t, cmd.Command, "profile.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestProblemContextCompleteness_Property is a property-based test for context completeness
// **Property 6: Problem Context Completeness**
// **Validates: Requirements 5.1, 5.3, 5.4, 5.5**
func TestProblemContextCompleteness_Property(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 100,
		MaxHotPaths:       100,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)
	generator := NewContextGenerator(analyzer)

	// Property: For any generated ProblemContext, it SHALL contain:
	// - Non-empty Title
	// - Valid Severity (critical/high/medium/low)
	// - Non-empty Impact string with percentage or size information
	// - At least one HotPath (if profile has samples)
	// - At least one ExecutableCmd

	// Title to profile type mapping to ensure consistency
	titleProfileMap := map[string]string{
		"CPU 热点":       "cpu",
		"内存泄漏":         "heap",
		"Goroutine 泄漏": "goroutine",
		"性能问题":         "cpu", // default to cpu
		"Heap 增长":      "heap",
	}

	f := func(titleSeed, severitySeed uint8) bool {
		// Generate test data based on seeds
		titles := []string{"CPU 热点", "内存泄漏", "Goroutine 泄漏", "性能问题", "Heap 增长"}
		severities := []string{"critical", "high", "medium", "low", "严重", "高", "中", "低", "unknown"}

		title := titles[int(titleSeed)%len(titles)]
		severity := severities[int(severitySeed)%len(severities)]
		// Use the correct profile type based on the title
		profileType := titleProfileMap[title]

		finding := createTestFinding(title, severity, []string{"建议1"})

		// Create a profile with samples
		funcNames := []string{
			"github.com/myapp/handler.ProcessRequest",
			"encoding/json.Marshal",
			"runtime.mallocgc",
		}
		p := createTestProfileWithSamples(funcNames, 1000)
		profiles := map[string]*profile.Profile{profileType: p}

		ctx := generator.GenerateContext(finding, profiles)

		if ctx == nil {
			t.Log("Context is nil")
			return false
		}

		// Property 1: Non-empty Title
		if ctx.Title == "" {
			t.Log("Title is empty")
			return false
		}

		// Property 2: Valid Severity (critical/high/medium/low)
		validSeverities := map[string]bool{
			"critical": true,
			"high":     true,
			"medium":   true,
			"low":      true,
		}
		if !validSeverities[ctx.Severity] {
			t.Logf("Invalid severity: %s", ctx.Severity)
			return false
		}

		// Property 3: Non-empty Impact string
		if ctx.Impact == "" {
			t.Log("Impact is empty")
			return false
		}

		// Property 4: At least one HotPath (if profile has samples)
		// Since we created a profile with samples, we should have hot paths
		if len(ctx.HotPaths) == 0 {
			t.Log("No hot paths found despite profile having samples")
			return false
		}

		// Property 5: At least one ExecutableCmd
		if len(ctx.Commands) == 0 {
			t.Log("No commands generated")
			return false
		}

		// Additional validation: Commands should start with "go tool pprof"
		for _, cmd := range ctx.Commands {
			if !strings.HasPrefix(cmd.Command, "go tool pprof") {
				t.Logf("Invalid command format: %s", cmd.Command)
				return false
			}
			if cmd.Description == "" {
				t.Log("Command description is empty")
				return false
			}
			if cmd.OutputHint == "" {
				t.Log("Command output hint is empty")
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProblemContextCompleteness_Property_EmptyProfile tests context with empty profile
// **Property 6: Problem Context Completeness (edge case)**
// **Validates: Requirements 5.1, 5.3**
func TestProblemContextCompleteness_Property_EmptyProfile(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 100,
		MaxHotPaths:       100,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)
	generator := NewContextGenerator(analyzer)

	// Property: Even with empty profile, context should have valid structure

	f := func(titleSeed, severitySeed uint8) bool {
		titles := []string{"CPU 热点", "内存泄漏", "Goroutine 泄漏"}
		severities := []string{"critical", "high", "medium", "low"}

		title := titles[int(titleSeed)%len(titles)]
		severity := severities[int(severitySeed)%len(severities)]

		finding := createTestFinding(title, severity, []string{"建议1"})

		// Empty profiles
		profiles := map[string]*profile.Profile{}

		ctx := generator.GenerateContext(finding, profiles)

		if ctx == nil {
			t.Log("Context is nil")
			return false
		}

		// Property 1: Non-empty Title
		if ctx.Title == "" {
			t.Log("Title is empty")
			return false
		}

		// Property 2: Valid Severity
		validSeverities := map[string]bool{
			"critical": true,
			"high":     true,
			"medium":   true,
			"low":      true,
		}
		if !validSeverities[ctx.Severity] {
			t.Logf("Invalid severity: %s", ctx.Severity)
			return false
		}

		// Property 3: Non-empty Explanation
		if ctx.Explanation == "" {
			t.Log("Explanation is empty")
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
