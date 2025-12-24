package main

import (
	"flag"
	"os"
	"testing"
	"testing/quick"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsProfileFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"cpu.pprof", true},
		{"heap.pprof", true},
		{"test.profile", true},
		{"readme.md", false},
		{"main.go", false},
		{"data.json", false},
		{".pprof", true},
		{"path/to/cpu.pprof", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isProfileFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProfilePaths_SingleFile(t *testing.T) {
	// 创建临时 pprof 文件
	tempFile, err := os.CreateTemp("", "test*.pprof")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	paths, err := getProfilePaths(tempFile.Name())
	require.NoError(t, err)
	assert.Len(t, paths, 1)
	assert.Equal(t, tempFile.Name(), paths[0])
}

func TestGetProfilePaths_Directory(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "perfinspector-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	file1, err := os.CreateTemp(tempDir, "cpu*.pprof")
	require.NoError(t, err)
	file1.Close()

	file2, err := os.CreateTemp(tempDir, "heap*.pprof")
	require.NoError(t, err)
	file2.Close()

	// 创建非 pprof 文件
	file3, err := os.CreateTemp(tempDir, "readme*.md")
	require.NoError(t, err)
	file3.Close()

	paths, err := getProfilePaths(tempDir)
	require.NoError(t, err)
	assert.Len(t, paths, 2) // 只有 .pprof 文件
}

func TestGetProfilePaths_NonExistent(t *testing.T) {
	_, err := getProfilePaths("/nonexistent/path")
	assert.Error(t, err)
}

func TestGetProfilePaths_InvalidFile(t *testing.T) {
	// 创建非 pprof 文件
	tempFile, err := os.CreateTemp("", "test*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	_, err = getProfilePaths(tempFile.Name())
	assert.Error(t, err)
}

// Feature: problem-locator, Property 9: Configuration Limits Respected
// Validates: Requirements 8.3, 8.4

// createTestProfileForMain creates a test profile with the given samples
func createTestProfileForMain(samples []*profile.Sample) *profile.Profile {
	return &profile.Profile{
		Sample: samples,
	}
}

// createTestSampleForMain creates a test sample with the given function names and value
func createTestSampleForMain(funcNames []string, value int64) *profile.Sample {
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
	return &profile.Sample{
		Location: locations,
		Value:    []int64{value},
	}
}

// TestConfigurationLimits_Property_MaxCallStackDepth tests that MaxCallStackDepth is respected
// **Property 9: Configuration Limits Respected**
// **Validates: Requirements 8.3, 8.4**
func TestConfigurationLimits_Property_MaxCallStackDepth(t *testing.T) {
	// Property: For any LocatorConfig with MaxCallStackDepth=N,
	// no CallChain SHALL have more than N frames displayed

	f := func(maxDepth uint8, numFrames uint8) bool {
		// Limit maxDepth to reasonable range (1-50)
		depth := int(maxDepth%50) + 1
		// Generate more frames than the limit
		frames := int(numFrames%30) + depth + 5

		config := locator.LocatorConfig{
			ModuleName:        "github.com/myapp",
			MaxCallStackDepth: depth,
			MaxHotPaths:       10,
		}

		classifier := locator.NewClassifier(config)
		extractor := locator.NewExtractor(classifier)
		analyzer := locator.NewPathAnalyzer(extractor, config)

		// Create sample with more frames than the limit
		funcNames := make([]string, frames)
		for i := 0; i < frames; i++ {
			funcNames[i] = "github.com/myapp/handler.Func" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		}

		sample := createTestSampleForMain(funcNames, 1000)
		p := createTestProfileForMain([]*profile.Sample{sample})

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")
		if len(hotPaths) == 0 {
			// No hot paths is acceptable
			return true
		}

		// Verify no call chain exceeds MaxCallStackDepth
		for _, hp := range hotPaths {
			if len(hp.Chain.Frames) > depth {
				t.Logf("CallChain has %d frames, exceeds MaxCallStackDepth=%d", len(hp.Chain.Frames), depth)
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

// TestConfigurationLimits_Property_MaxHotPaths tests that MaxHotPaths is respected
// **Property 9: Configuration Limits Respected**
// **Validates: Requirements 8.3, 8.4**
func TestConfigurationLimits_Property_MaxHotPaths(t *testing.T) {
	// Property: For any LocatorConfig with MaxHotPaths=M,
	// no analysis result SHALL have more than M HotPaths

	f := func(maxPaths uint8, numSamples uint8) bool {
		// Limit maxPaths to reasonable range (1-20)
		paths := int(maxPaths%20) + 1
		// Generate more samples than the limit
		samples := int(numSamples%30) + paths + 5

		config := locator.LocatorConfig{
			ModuleName:        "github.com/myapp",
			MaxCallStackDepth: 10,
			MaxHotPaths:       paths,
		}

		classifier := locator.NewClassifier(config)
		extractor := locator.NewExtractor(classifier)
		analyzer := locator.NewPathAnalyzer(extractor, config)

		// Create multiple samples with different call paths
		profileSamples := make([]*profile.Sample, samples)
		for i := 0; i < samples; i++ {
			// Each sample has a unique call path
			funcNames := []string{
				"github.com/myapp/handler.Handler" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
				"encoding/json.Marshal",
				"runtime.mallocgc",
			}
			profileSamples[i] = createTestSampleForMain(funcNames, int64(1000-i))
		}

		p := createTestProfileForMain(profileSamples)

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

		// Verify no more than MaxHotPaths are returned
		if len(hotPaths) > paths {
			t.Logf("Got %d hot paths, exceeds MaxHotPaths=%d", len(hotPaths), paths)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestCreateLocatorConfig tests the createLocatorConfig function
func TestCreateLocatorConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		config := &Config{
			StackDepth: 10,
			HotPaths:   5,
		}
		locatorConfig := createLocatorConfig(config)

		assert.Equal(t, 10, locatorConfig.MaxCallStackDepth)
		assert.Equal(t, 5, locatorConfig.MaxHotPaths)
	})

	t.Run("custom module name", func(t *testing.T) {
		config := &Config{
			ModuleName: "github.com/custom/module",
			StackDepth: 10,
			HotPaths:   5,
		}
		locatorConfig := createLocatorConfig(config)

		assert.Equal(t, "github.com/custom/module", locatorConfig.ModuleName)
	})

	t.Run("third party prefixes", func(t *testing.T) {
		config := &Config{
			ThirdPartyPrefixes: []string{"github.com/vendor1", "github.com/vendor2"},
			StackDepth:         10,
			HotPaths:           5,
		}
		locatorConfig := createLocatorConfig(config)

		assert.Equal(t, []string{"github.com/vendor1", "github.com/vendor2"}, locatorConfig.ThirdPartyPrefixes)
	})

	t.Run("custom limits", func(t *testing.T) {
		config := &Config{
			StackDepth: 20,
			HotPaths:   15,
		}
		locatorConfig := createLocatorConfig(config)

		assert.Equal(t, 20, locatorConfig.MaxCallStackDepth)
		assert.Equal(t, 15, locatorConfig.MaxHotPaths)
	})
}

// TestParseArgs_LocatorOptions tests parsing of locator-related command line options
func TestParseArgs_LocatorOptions(t *testing.T) {
	// Save original args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("default locator options", func(t *testing.T) {
		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Create a temp file to use as input
		tempFile, err := os.CreateTemp("", "test*.pprof")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		os.Args = []string{"cmd", tempFile.Name()}
		config, err := parseArgs()
		require.NoError(t, err)

		assert.Equal(t, "", config.ModuleName)
		assert.Nil(t, config.ThirdPartyPrefixes)
		assert.Equal(t, 10, config.StackDepth)
		assert.Equal(t, 5, config.HotPaths)
	})

	t.Run("custom locator options", func(t *testing.T) {
		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Create a temp file to use as input
		tempFile, err := os.CreateTemp("", "test*.pprof")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		os.Args = []string{
			"cmd",
			"-module", "github.com/myorg/myapp",
			"-third-party-prefixes", "github.com/vendor1,github.com/vendor2",
			"-stack-depth", "15",
			"-hot-paths", "10",
			tempFile.Name(),
		}
		config, err := parseArgs()
		require.NoError(t, err)

		assert.Equal(t, "github.com/myorg/myapp", config.ModuleName)
		assert.Equal(t, []string{"github.com/vendor1", "github.com/vendor2"}, config.ThirdPartyPrefixes)
		assert.Equal(t, 15, config.StackDepth)
		assert.Equal(t, 10, config.HotPaths)
	})

	t.Run("stack depth limits", func(t *testing.T) {
		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Create a temp file to use as input
		tempFile, err := os.CreateTemp("", "test*.pprof")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		// Test minimum limit
		os.Args = []string{"cmd", "-stack-depth", "0", tempFile.Name()}
		config, err := parseArgs()
		require.NoError(t, err)
		assert.Equal(t, 1, config.StackDepth) // Should be clamped to 1

		// Reset flags for next test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Test maximum limit
		os.Args = []string{"cmd", "-stack-depth", "200", tempFile.Name()}
		config, err = parseArgs()
		require.NoError(t, err)
		assert.Equal(t, 100, config.StackDepth) // Should be clamped to 100
	})

	t.Run("hot paths limits", func(t *testing.T) {
		// Reset flags for this test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Create a temp file to use as input
		tempFile, err := os.CreateTemp("", "test*.pprof")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		// Test minimum limit
		os.Args = []string{"cmd", "-hot-paths", "0", tempFile.Name()}
		config, err := parseArgs()
		require.NoError(t, err)
		assert.Equal(t, 1, config.HotPaths) // Should be clamped to 1

		// Reset flags for next test
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Test maximum limit
		os.Args = []string{"cmd", "-hot-paths", "100", tempFile.Name()}
		config, err = parseArgs()
		require.NoError(t, err)
		assert.Equal(t, 50, config.HotPaths) // Should be clamped to 50
	})
}
