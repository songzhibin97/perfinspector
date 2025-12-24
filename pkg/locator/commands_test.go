package locator

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

// Feature: problem-locator, Property 7: Command Generation Validity
// Validates: Requirements 6.1, 6.2, 6.3, 6.4

// TestNewCommandGenerator tests the constructor
func TestNewCommandGenerator(t *testing.T) {
	generator := NewCommandGenerator()
	assert.NotNil(t, generator)
}

// TestGenerateCommands_Basic tests basic command generation
func TestGenerateCommands_Basic(t *testing.T) {
	generator := NewCommandGenerator()
	commands := generator.GenerateCommands("./cpu.pprof", "cpu", nil)

	// Should have at least top and web commands
	assert.True(t, len(commands) >= 2)

	// All commands should start with "go tool pprof"
	for _, cmd := range commands {
		assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
		assert.NotEmpty(t, cmd.Description)
		assert.NotEmpty(t, cmd.OutputHint)
	}
}

// TestGenerateCommands_WithHotPaths tests command generation with hot paths
func TestGenerateCommands_WithHotPaths(t *testing.T) {
	generator := NewCommandGenerator()

	hotPaths := []HotPath{
		{
			Chain: CallChain{
				Frames: []StackFrame{
					{FunctionName: "main.main", ShortName: "main", Category: CategoryBusiness},
					{FunctionName: "runtime.mallocgc", ShortName: "mallocgc", Category: CategoryRuntime},
				},
			},
			RootCauseIndex: 0,
		},
	}

	commands := generator.GenerateCommands("./cpu.pprof", "cpu", hotPaths)

	// Should have top, focus, list, and web commands
	assert.True(t, len(commands) >= 4)

	// Check for focus command
	hasFocus := false
	hasList := false
	for _, cmd := range commands {
		if strings.Contains(cmd.Command, "-focus=") {
			hasFocus = true
		}
		if strings.Contains(cmd.Command, "-list=") {
			hasList = true
		}
	}
	assert.True(t, hasFocus, "Should have focus command")
	assert.True(t, hasList, "Should have list command")
}

// TestGenerateCommands_EmptyProfilePath tests command generation with empty profile path
func TestGenerateCommands_EmptyProfilePath(t *testing.T) {
	generator := NewCommandGenerator()
	commands := generator.GenerateCommands("", "cpu", nil)

	// Should still generate commands with default path
	assert.True(t, len(commands) >= 2)

	// Commands should contain a default path
	for _, cmd := range commands {
		assert.Contains(t, cmd.Command, ".pprof")
	}
}

// TestCommandGenerator_GenerateFocusCommand tests focus command generation
func TestCommandGenerator_GenerateFocusCommand(t *testing.T) {
	generator := NewCommandGenerator()

	tests := []struct {
		name         string
		profilePath  string
		functionName string
	}{
		{"simple function", "./cpu.pprof", "HandleRequest"},
		{"method", "./heap.pprof", "(*Server).ServeHTTP"},
		{"full path", "/path/to/profile.pprof", "main.main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generator.GenerateFocusCommand(tt.profilePath, tt.functionName)

			assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
			assert.Contains(t, cmd.Command, "-focus=")
			assert.Contains(t, cmd.Command, tt.profilePath)
			assert.NotEmpty(t, cmd.Description)
			assert.NotEmpty(t, cmd.OutputHint)
		})
	}
}

// TestCommandGenerator_GenerateTopCommand tests top command generation
func TestCommandGenerator_GenerateTopCommand(t *testing.T) {
	generator := NewCommandGenerator()

	tests := []struct {
		name        string
		profilePath string
	}{
		{"relative path", "./cpu.pprof"},
		{"absolute path", "/path/to/profile.pprof"},
		{"simple name", "profile.pprof"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generator.GenerateTopCommand(tt.profilePath)

			assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
			assert.Contains(t, cmd.Command, "-top")
			assert.Contains(t, cmd.Command, tt.profilePath)
			assert.NotEmpty(t, cmd.Description)
			assert.NotEmpty(t, cmd.OutputHint)
		})
	}
}

// TestCommandGenerator_GenerateListCommand tests list command generation
func TestCommandGenerator_GenerateListCommand(t *testing.T) {
	generator := NewCommandGenerator()

	tests := []struct {
		name         string
		profilePath  string
		functionName string
	}{
		{"simple function", "./cpu.pprof", "HandleRequest"},
		{"method", "./heap.pprof", "ServeHTTP"},
		{"full path", "/path/to/profile.pprof", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generator.GenerateListCommand(tt.profilePath, tt.functionName)

			assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
			assert.Contains(t, cmd.Command, "-list=")
			assert.Contains(t, cmd.Command, tt.profilePath)
			assert.NotEmpty(t, cmd.Description)
			assert.NotEmpty(t, cmd.OutputHint)
		})
	}
}

// TestCommandGenerator_GenerateWebCommand tests web command generation
func TestCommandGenerator_GenerateWebCommand(t *testing.T) {
	generator := NewCommandGenerator()

	tests := []struct {
		name        string
		profilePath string
	}{
		{"relative path", "./cpu.pprof"},
		{"absolute path", "/path/to/profile.pprof"},
		{"simple name", "profile.pprof"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generator.GenerateWebCommand(tt.profilePath)

			assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
			assert.Contains(t, cmd.Command, "-http=")
			assert.Contains(t, cmd.Command, tt.profilePath)
			assert.NotEmpty(t, cmd.Description)
			assert.NotEmpty(t, cmd.OutputHint)
		})
	}
}

// TestGenerateAllocSpaceCommand tests alloc_space command generation
func TestGenerateAllocSpaceCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateAllocSpaceCommand("./heap.pprof")

	assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
	assert.Contains(t, cmd.Command, "-alloc_space")
	assert.Contains(t, cmd.Command, "./heap.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestGenerateInuseSpaceCommand tests inuse_space command generation
func TestGenerateInuseSpaceCommand(t *testing.T) {
	generator := NewCommandGenerator()
	cmd := generator.GenerateInuseSpaceCommand("./heap.pprof")

	assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
	assert.Contains(t, cmd.Command, "-inuse_space")
	assert.Contains(t, cmd.Command, "./heap.pprof")
	assert.NotEmpty(t, cmd.Description)
	assert.NotEmpty(t, cmd.OutputHint)
}

// TestGenerateCommandsForProfileType_Heap tests heap-specific commands
func TestGenerateCommandsForProfileType_Heap(t *testing.T) {
	generator := NewCommandGenerator()
	commands := generator.GenerateCommandsForProfileType("./heap.pprof", "heap", nil)

	// Should have heap-specific commands
	hasAllocSpace := false
	hasInuseSpace := false
	for _, cmd := range commands {
		if strings.Contains(cmd.Command, "-alloc_space") {
			hasAllocSpace = true
		}
		if strings.Contains(cmd.Command, "-inuse_space") {
			hasInuseSpace = true
		}
	}
	assert.True(t, hasAllocSpace, "Should have alloc_space command for heap profile")
	assert.True(t, hasInuseSpace, "Should have inuse_space command for heap profile")
}

// TestExtractShortFunctionName tests function name extraction
func TestExtractShortFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HandleRequest", "HandleRequest"},
		{"main.HandleRequest", "HandleRequest"},
		{"github.com/user/pkg.HandleRequest", "HandleRequest"},
		{"github.com/user/pkg.(*Type).Method", "Method"},
		{"", ""},
		// 匿名函数测试用例
		{"main.init.0.func1.1", "init.0.func1.1"},
		{"main.createWorker.func1", "createWorker.func1"},
		{"main.init.func1", "init.func1"},
		{"github.com/user/pkg.(*Server).handleRequest.func1", "handleRequest.func1"},
		{"main.main.func1", "main.func1"},
		{"runtime.main.func1", "main.func1"},
		// 带数字后缀的匿名函数
		{"main.init.0.func1.2", "init.0.func1.2"},
		{"main.TestFunc.func1.1.1", "TestFunc.func1.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractShortFunctionName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsAnonymousFunction tests anonymous function detection
func TestIsAnonymousFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"main.HandleRequest", false},
		{"main.init.func1", true},
		{"main.createWorker.func1", true},
		{"main.init.0.func1.1", true},
		{"github.com/user/pkg.(*Server).handleRequest.func1", true},
		{"runtime.mallocgc", false},
		{"main.main", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isAnonymousFunction(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractAnonymousFunctionName tests anonymous function name extraction
func TestExtractAnonymousFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"main.init.func1", "init.func1"},
		{"main.createWorker.func1", "createWorker.func1"},
		{"main.init.0.func1.1", "init.0.func1.1"},
		{"github.com/user/pkg.(*Server).handleRequest.func1", "handleRequest.func1"},
		{"main.main.func1", "main.func1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractAnonymousFunctionName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCommandGenerationValidity_Property is a property-based test for command generation
// **Property 7: Command Generation Validity**
// **Validates: Requirements 6.1, 6.2, 6.3, 6.4**
func TestCommandGenerationValidity_Property(t *testing.T) {
	generator := NewCommandGenerator()

	// Property: For any generated ExecutableCmd, the Command string SHALL:
	// - Start with "go tool pprof"
	// - Contain a valid profile file path placeholder or actual path
	// - Include -focus flag when function name is provided
	// - Have non-empty Description and OutputHint

	f := func(profilePathSeed, functionNameSeed, profileTypeSeed uint8) bool {
		// Generate test data based on seeds
		profilePaths := []string{
			"./cpu.pprof",
			"./heap.pprof",
			"./goroutine.pprof",
			"/path/to/profile.pprof",
			"profile.pprof",
		}
		functionNames := []string{
			"HandleRequest",
			"main.main",
			"(*Server).ServeHTTP",
			"github.com/user/pkg.Process",
			"runtime.mallocgc",
		}
		profileTypes := []string{"cpu", "heap", "goroutine"}

		profilePath := profilePaths[int(profilePathSeed)%len(profilePaths)]
		functionName := functionNames[int(functionNameSeed)%len(functionNames)]
		profileType := profileTypes[int(profileTypeSeed)%len(profileTypes)]

		// Test GenerateFocusCommand
		focusCmd := generator.GenerateFocusCommand(profilePath, functionName)
		if !validateCommand(focusCmd, profilePath, true) {
			t.Logf("Focus command validation failed: %+v", focusCmd)
			return false
		}

		// Test GenerateTopCommand
		topCmd := generator.GenerateTopCommand(profilePath)
		if !validateCommand(topCmd, profilePath, false) {
			t.Logf("Top command validation failed: %+v", topCmd)
			return false
		}

		// Test GenerateListCommand
		listCmd := generator.GenerateListCommand(profilePath, functionName)
		if !validateCommand(listCmd, profilePath, false) {
			t.Logf("List command validation failed: %+v", listCmd)
			return false
		}
		if !strings.Contains(listCmd.Command, "-list=") {
			t.Log("List command missing -list flag")
			return false
		}

		// Test GenerateWebCommand
		webCmd := generator.GenerateWebCommand(profilePath)
		if !validateCommand(webCmd, profilePath, false) {
			t.Logf("Web command validation failed: %+v", webCmd)
			return false
		}
		if !strings.Contains(webCmd.Command, "-http=") {
			t.Log("Web command missing -http flag")
			return false
		}

		// Test GenerateCommands with hot paths
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: functionName, ShortName: extractShortFunctionName(functionName), Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
				ProfileType:    profileType,
			},
		}
		commands := generator.GenerateCommands(profilePath, profileType, hotPaths)

		// Validate all generated commands
		for _, cmd := range commands {
			if !strings.HasPrefix(cmd.Command, "go tool pprof") {
				t.Logf("Command doesn't start with 'go tool pprof': %s", cmd.Command)
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

// validateCommand validates a single ExecutableCmd
func validateCommand(cmd ExecutableCmd, profilePath string, expectFocus bool) bool {
	// Must start with "go tool pprof"
	if !strings.HasPrefix(cmd.Command, "go tool pprof") {
		return false
	}

	// Must contain profile path
	if !strings.Contains(cmd.Command, profilePath) {
		return false
	}

	// If focus is expected, must have -focus flag
	if expectFocus && !strings.Contains(cmd.Command, "-focus=") {
		return false
	}

	// Must have non-empty Description
	if cmd.Description == "" {
		return false
	}

	// Must have non-empty OutputHint
	if cmd.OutputHint == "" {
		return false
	}

	return true
}

// TestCommandGenerationValidity_Property_NoHotPaths tests command generation without hot paths
// **Property 7: Command Generation Validity (edge case)**
// **Validates: Requirements 6.1, 6.2**
func TestCommandGenerationValidity_Property_NoHotPaths(t *testing.T) {
	generator := NewCommandGenerator()

	// Property: Even without hot paths, basic commands should be generated

	f := func(profilePathSeed, profileTypeSeed uint8) bool {
		profilePaths := []string{
			"./cpu.pprof",
			"./heap.pprof",
			"./goroutine.pprof",
		}
		profileTypes := []string{"cpu", "heap", "goroutine"}

		profilePath := profilePaths[int(profilePathSeed)%len(profilePaths)]
		profileType := profileTypes[int(profileTypeSeed)%len(profileTypes)]

		commands := generator.GenerateCommands(profilePath, profileType, nil)

		// Should have at least 2 commands (top and web)
		if len(commands) < 2 {
			t.Logf("Expected at least 2 commands, got %d", len(commands))
			return false
		}

		// All commands should be valid
		for _, cmd := range commands {
			if !strings.HasPrefix(cmd.Command, "go tool pprof") {
				t.Logf("Command doesn't start with 'go tool pprof': %s", cmd.Command)
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

// Feature: report-improvements, Property 6: Heap Profile 命令包含内存选项
// **Property 6: Heap Profile 命令包含内存选项**
// **Validates: Requirements 3.1**
// *For any* profileType 为 "heap" 的调用，GenerateCommandsForProfileType() 返回的命令列表 SHALL 包含 "-alloc_space" 和 "-inuse_space" 选项
func TestProperty6_HeapProfileCommandsContainMemoryOptions(t *testing.T) {
	generator := NewCommandGenerator()

	f := func(profilePathSeed uint8) bool {
		profilePaths := []string{
			"./heap.pprof",
			"/path/to/heap.pprof",
			"testdata/heap1.pprof",
			"heap_profile.pprof",
			"./profiles/heap.pprof",
		}
		profilePath := profilePaths[int(profilePathSeed)%len(profilePaths)]

		// Generate commands for heap profile type
		commands := generator.GenerateCommandsForProfileType(profilePath, "heap", nil)

		// Check for alloc_space and inuse_space options
		hasAllocSpace := false
		hasInuseSpace := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-alloc_space") {
				hasAllocSpace = true
			}
			if strings.Contains(cmd.Command, "-inuse_space") {
				hasInuseSpace = true
			}
		}

		if !hasAllocSpace {
			t.Logf("Missing -alloc_space option for heap profile: %s", profilePath)
			return false
		}
		if !hasInuseSpace {
			t.Logf("Missing -inuse_space option for heap profile: %s", profilePath)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: report-improvements, Property 7: 热点函数生成 Focus 命令
// **Property 7: 热点函数生成 Focus 命令**
// **Validates: Requirements 3.2, 3.3**
// *For any* 包含 RootCauseIndex >= 0 的 HotPath，GenerateCommands() 返回的命令列表 SHALL 包含 "-focus=<function_name>" 选项
func TestProperty7_HotPathGeneratesFocusCommand(t *testing.T) {
	generator := NewCommandGenerator()

	f := func(functionNameSeed, profileTypeSeed uint8) bool {
		// Use function names that are already short names (no package prefix)
		// to avoid confusion with extractShortFunctionName behavior
		functionNames := []string{
			"HandleRequest",
			"ProcessData",
			"ServeHTTP",
			"Execute",
			"Run",
		}
		profileTypes := []string{"cpu", "heap", "goroutine"}

		functionName := functionNames[int(functionNameSeed)%len(functionNames)]
		profileType := profileTypes[int(profileTypeSeed)%len(profileTypes)]
		profilePath := "./" + profileType + ".pprof"

		// Create hot path with valid RootCauseIndex
		// ShortName should be the extracted short function name
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{
							FunctionName: "main." + functionName,
							ShortName:    functionName, // This is the short name used in focus command
							Category:     CategoryBusiness,
						},
					},
				},
				RootCauseIndex: 0, // Valid index >= 0
				ProfileType:    profileType,
			},
		}

		commands := generator.GenerateCommands(profilePath, profileType, hotPaths)

		// Check for focus command with the short function name
		hasFocus := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-focus="+functionName) {
				hasFocus = true
				break
			}
		}

		if !hasFocus {
			t.Logf("Missing -focus=%s command for function %s", functionName, functionName)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: report-improvements, Property 8: 多文件生成 Diff 命令
// **Property 8: 多文件生成 Diff 命令**
// **Validates: Requirements 3.4**
// *For any* 包含多个 profile 路径的调用，GenerateCommandsWithContext() 返回的命令列表 SHALL 包含 "-base" 选项
func TestProperty8_MultipleFilesGenerateDiffCommand(t *testing.T) {
	generator := NewCommandGenerator()

	f := func(profileTypeSeed, pathCountSeed uint8) bool {
		profileTypes := []string{"cpu", "heap", "goroutine"}
		profileType := profileTypes[int(profileTypeSeed)%len(profileTypes)]

		// Generate 2-5 profile paths
		pathCount := 2 + int(pathCountSeed)%4
		profilePaths := make([]string, pathCount)
		for i := 0; i < pathCount; i++ {
			profilePaths[i] = fmt.Sprintf("./testdata/%s_%d.pprof", profileType, i+1)
		}

		commands := generator.GenerateCommandsWithContext(profilePaths, profileType, nil)

		// Check for -base option
		hasBase := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-base=") {
				hasBase = true
				break
			}
		}

		if !hasBase {
			t.Logf("Missing -base option for multiple profile paths: %v", profilePaths)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: report-improvements, Property 9: 命令使用实际路径且有中文说明
// **Property 9: 命令使用实际路径且有中文说明**
// **Validates: Requirements 3.5, 3.6**
// *For any* 生成的 ExecutableCmd，Command 字段 SHALL 包含传入的 profilePath，且 Description 字段 SHALL 非空
func TestProperty9_CommandsUseActualPathsAndHaveChineseDescription(t *testing.T) {
	generator := NewCommandGenerator()

	f := func(profilePathSeed, profileTypeSeed uint8) bool {
		profilePaths := []string{
			"./testdata/profiles/cpu1.pprof",
			"/absolute/path/to/heap.pprof",
			"relative/goroutine.pprof",
			"./profiles/scenario1/heap_1.pprof",
			"testdata/demo/cpu.pprof",
		}
		profileTypes := []string{"cpu", "heap", "goroutine"}

		profilePath := profilePaths[int(profilePathSeed)%len(profilePaths)]
		profileType := profileTypes[int(profileTypeSeed)%len(profileTypes)]

		// Test single path commands
		commands := generator.GenerateCommandsWithContext([]string{profilePath}, profileType, nil)

		for _, cmd := range commands {
			// Check that command contains the actual profile path
			if !strings.Contains(cmd.Command, profilePath) {
				t.Logf("Command does not contain actual path %s: %s", profilePath, cmd.Command)
				return false
			}

			// Check that description is non-empty (Chinese description)
			if cmd.Description == "" {
				t.Logf("Command description is empty for: %s", cmd.Command)
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

// TestGenerateDiffCommand tests the diff command generation
func TestGenerateDiffCommand(t *testing.T) {
	generator := NewCommandGenerator()

	tests := []struct {
		name       string
		basePath   string
		targetPath string
	}{
		{"relative paths", "./heap1.pprof", "./heap2.pprof"},
		{"absolute paths", "/path/to/base.pprof", "/path/to/target.pprof"},
		{"mixed paths", "./base.pprof", "/absolute/target.pprof"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generator.GenerateDiffCommand(tt.basePath, tt.targetPath)

			assert.True(t, strings.HasPrefix(cmd.Command, "go tool pprof"))
			assert.Contains(t, cmd.Command, "-base="+tt.basePath)
			assert.Contains(t, cmd.Command, tt.targetPath)
			assert.NotEmpty(t, cmd.Description)
			assert.NotEmpty(t, cmd.OutputHint)
		})
	}
}

// TestGenerateCommandsWithContext tests the context-aware command generation
func TestGenerateCommandsWithContext(t *testing.T) {
	generator := NewCommandGenerator()

	t.Run("single profile path", func(t *testing.T) {
		commands := generator.GenerateCommandsWithContext(
			[]string{"./cpu.pprof"},
			"cpu",
			nil,
		)

		assert.True(t, len(commands) >= 2) // top and web commands
		for _, cmd := range commands {
			assert.Contains(t, cmd.Command, "./cpu.pprof")
		}
	})

	t.Run("multiple profile paths generates diff", func(t *testing.T) {
		commands := generator.GenerateCommandsWithContext(
			[]string{"./heap1.pprof", "./heap2.pprof", "./heap3.pprof"},
			"heap",
			nil,
		)

		hasDiff := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-base=") {
				hasDiff = true
				assert.Contains(t, cmd.Command, "./heap1.pprof") // base
				assert.Contains(t, cmd.Command, "./heap3.pprof") // target (last)
				break
			}
		}
		assert.True(t, hasDiff, "Should have diff command for multiple profiles")
	})

	t.Run("heap profile has memory options", func(t *testing.T) {
		commands := generator.GenerateCommandsWithContext(
			[]string{"./heap.pprof"},
			"heap",
			nil,
		)

		hasAllocSpace := false
		hasInuseSpace := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-alloc_space") {
				hasAllocSpace = true
			}
			if strings.Contains(cmd.Command, "-inuse_space") {
				hasInuseSpace = true
			}
		}
		assert.True(t, hasAllocSpace, "Should have alloc_space command")
		assert.True(t, hasInuseSpace, "Should have inuse_space command")
	})

	t.Run("with hot paths generates focus command", func(t *testing.T) {
		hotPaths := []HotPath{
			{
				Chain: CallChain{
					Frames: []StackFrame{
						{FunctionName: "main.HandleRequest", ShortName: "HandleRequest", Category: CategoryBusiness},
					},
				},
				RootCauseIndex: 0,
				ProfileType:    "cpu",
			},
		}

		commands := generator.GenerateCommandsWithContext(
			[]string{"./cpu.pprof"},
			"cpu",
			hotPaths,
		)

		hasFocus := false
		for _, cmd := range commands {
			if strings.Contains(cmd.Command, "-focus=HandleRequest") {
				hasFocus = true
				break
			}
		}
		assert.True(t, hasFocus, "Should have focus command for hot path")
	})
}
