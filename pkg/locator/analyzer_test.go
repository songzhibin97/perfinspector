package locator

import (
	"testing"
	"testing/quick"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
)

// Feature: problem-locator, Property 3: Call Chain Completeness and Ordering
// Feature: problem-locator, Property 4: Hot Path Aggregation Correctness
// Feature: problem-locator, Property 5: Business Frame Identification
// Validates: Requirements 3.1, 3.2, 3.4, 4.1, 4.2, 4.4, 4.6

// createTestProfile creates a test profile with the given samples
func createTestProfile(samples []*profile.Sample) *profile.Profile {
	return &profile.Profile{
		Sample: samples,
	}
}

// createTestSample creates a test sample with the given function names and value
func createTestSample(funcNames []string, value int64, classifier *Classifier) *profile.Sample {
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

// TestAnalyzeHotPaths_Basic tests basic hot path analysis
func TestAnalyzeHotPaths_Basic(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Create a profile with samples
	funcNames := []string{
		"github.com/myapp/handler.ProcessRequest",
		"encoding/json.Marshal",
		"runtime.mallocgc",
	}
	sample := createTestSample(funcNames, 1000, classifier)
	p := createTestProfile([]*profile.Sample{sample})

	hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

	assert.Equal(t, 1, len(hotPaths))
	assert.Equal(t, "cpu", hotPaths[0].ProfileType)
	assert.Equal(t, 3, len(hotPaths[0].Chain.Frames))
}

// TestAnalyzeHotPaths_EmptyProfile tests hot path analysis with empty profile
func TestAnalyzeHotPaths_EmptyProfile(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	t.Run("nil profile", func(t *testing.T) {
		hotPaths := analyzer.AnalyzeHotPaths(nil, "cpu")
		assert.Nil(t, hotPaths)
	})

	t.Run("empty samples", func(t *testing.T) {
		p := createTestProfile([]*profile.Sample{})
		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")
		assert.Nil(t, hotPaths)
	})
}

// TestAnalyzeHotPaths_MaxHotPaths tests that MaxHotPaths limit is respected
func TestAnalyzeHotPaths_MaxHotPaths(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       3, // Limit to 3
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Create 5 different samples
	samples := make([]*profile.Sample, 5)
	for i := 0; i < 5; i++ {
		funcNames := []string{
			"github.com/myapp/handler.Handler" + string(rune('A'+i)),
			"runtime.mallocgc",
		}
		samples[i] = createTestSample(funcNames, int64(1000-i*100), classifier)
	}
	p := createTestProfile(samples)

	hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

	// Should only return MaxHotPaths (3) paths
	assert.Equal(t, 3, len(hotPaths))
}

// TestAnalyzeHotPaths_MaxCallStackDepth tests that MaxCallStackDepth limit is respected
func TestAnalyzeHotPaths_MaxCallStackDepth(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 3, // Limit to 3 frames
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Create a sample with 5 frames
	funcNames := []string{
		"github.com/myapp/handler.Handler1",
		"github.com/myapp/handler.Handler2",
		"github.com/myapp/handler.Handler3",
		"github.com/myapp/handler.Handler4",
		"github.com/myapp/handler.Handler5",
	}
	sample := createTestSample(funcNames, 1000, classifier)
	p := createTestProfile([]*profile.Sample{sample})

	hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

	assert.Equal(t, 1, len(hotPaths))
	// Should be limited to MaxCallStackDepth (3) frames
	assert.Equal(t, 3, len(hotPaths[0].Chain.Frames))
}

// TestAggregateCallChains_Basic tests basic call chain aggregation
func TestAggregateCallChains_Basic(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Create two identical call chains
	chain1 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "main.main", Category: CategoryBusiness},
			{FunctionName: "runtime.mallocgc", Category: CategoryRuntime},
		},
		TotalValue:        1000,
		TotalPct:          10.0,
		SampleCount:       1,
		CategoryBreakdown: map[CodeCategory]int{CategoryBusiness: 1, CategoryRuntime: 1},
		BoundaryPoints:    []int{1},
	}
	chain2 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "main.main", Category: CategoryBusiness},
			{FunctionName: "runtime.mallocgc", Category: CategoryRuntime},
		},
		TotalValue:        2000,
		TotalPct:          20.0,
		SampleCount:       1,
		CategoryBreakdown: map[CodeCategory]int{CategoryBusiness: 1, CategoryRuntime: 1},
		BoundaryPoints:    []int{1},
	}

	aggregated := analyzer.AggregateCallChains([]CallChain{chain1, chain2})

	// Should aggregate into one chain
	assert.Equal(t, 1, len(aggregated))
	// TotalValue should be sum
	assert.Equal(t, int64(3000), aggregated[0].TotalValue)
	// SampleCount should be sum
	assert.Equal(t, 2, aggregated[0].SampleCount)
}

// TestAggregateCallChains_DifferentPaths tests that different paths are not aggregated
func TestAggregateCallChains_DifferentPaths(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Create two different call chains
	chain1 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "main.main", Category: CategoryBusiness},
			{FunctionName: "runtime.mallocgc", Category: CategoryRuntime},
		},
		TotalValue:        1000,
		TotalPct:          10.0,
		SampleCount:       1,
		CategoryBreakdown: map[CodeCategory]int{CategoryBusiness: 1, CategoryRuntime: 1},
		BoundaryPoints:    []int{1},
	}
	chain2 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "main.other", Category: CategoryBusiness}, // Different function
			{FunctionName: "runtime.mallocgc", Category: CategoryRuntime},
		},
		TotalValue:        2000,
		TotalPct:          20.0,
		SampleCount:       1,
		CategoryBreakdown: map[CodeCategory]int{CategoryBusiness: 1, CategoryRuntime: 1},
		BoundaryPoints:    []int{1},
	}

	aggregated := analyzer.AggregateCallChains([]CallChain{chain1, chain2})

	// Should not aggregate - different paths
	assert.Equal(t, 2, len(aggregated))
}

// TestAggregateCallChains_Empty tests aggregation with empty input
func TestAggregateCallChains_Empty(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	aggregated := analyzer.AggregateCallChains(nil)
	assert.Nil(t, aggregated)

	aggregated = analyzer.AggregateCallChains([]CallChain{})
	assert.Nil(t, aggregated)
}

// TestFindBoundaryPoints tests boundary point detection
func TestFindBoundaryPoints(t *testing.T) {
	t.Run("no boundaries", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
		}
		boundaries := FindBoundaryPoints(frames)
		assert.Equal(t, 0, len(boundaries))
	})

	t.Run("single boundary", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryRuntime},
			{Category: CategoryRuntime},
		}
		boundaries := FindBoundaryPoints(frames)
		assert.Equal(t, []int{1}, boundaries)
	})

	t.Run("multiple boundaries", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryStdlib},
			{Category: CategoryRuntime},
		}
		boundaries := FindBoundaryPoints(frames)
		assert.Equal(t, []int{1, 2}, boundaries)
	})

	t.Run("empty frames", func(t *testing.T) {
		boundaries := FindBoundaryPoints(nil)
		assert.Nil(t, boundaries)

		boundaries = FindBoundaryPoints([]StackFrame{})
		assert.Nil(t, boundaries)
	})

	t.Run("single frame", func(t *testing.T) {
		frames := []StackFrame{{Category: CategoryBusiness}}
		boundaries := FindBoundaryPoints(frames)
		assert.Nil(t, boundaries)
	})
}

// TestFindBusinessFrames tests business frame identification
func TestFindBusinessFrames(t *testing.T) {
	t.Run("no business frames", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryRuntime},
			{Category: CategoryStdlib},
			{Category: CategoryRuntime},
		}
		businessFrames := FindBusinessFrames(frames)
		assert.Equal(t, 0, len(businessFrames))
	})

	t.Run("all business frames", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
		}
		businessFrames := FindBusinessFrames(frames)
		assert.Equal(t, []int{0, 1, 2}, businessFrames)
	})

	t.Run("mixed frames", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryStdlib},
			{Category: CategoryBusiness},
			{Category: CategoryRuntime},
		}
		businessFrames := FindBusinessFrames(frames)
		assert.Equal(t, []int{0, 2}, businessFrames)
	})

	t.Run("empty frames", func(t *testing.T) {
		businessFrames := FindBusinessFrames(nil)
		assert.Equal(t, 0, len(businessFrames))

		businessFrames = FindBusinessFrames([]StackFrame{})
		assert.Equal(t, 0, len(businessFrames))
	})
}

// TestGenerateCategorySummary tests category summary generation
func TestGenerateCategorySummary(t *testing.T) {
	t.Run("empty frames", func(t *testing.T) {
		summary := GenerateCategorySummary(nil)
		assert.Equal(t, "空调用链", summary)

		summary = GenerateCategorySummary([]StackFrame{})
		assert.Equal(t, "空调用链", summary)
	})

	t.Run("single category", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
		}
		summary := GenerateCategorySummary(frames)
		assert.Equal(t, "2 业务", summary)
	})

	t.Run("multiple categories", func(t *testing.T) {
		frames := []StackFrame{
			{Category: CategoryBusiness},
			{Category: CategoryBusiness},
			{Category: CategoryStdlib},
			{Category: CategoryRuntime},
			{Category: CategoryRuntime},
			{Category: CategoryRuntime},
		}
		summary := GenerateCategorySummary(frames)
		assert.Equal(t, "2 业务 → 1 标准库 → 3 运行时", summary)
	})
}

// TestCallChain_Property_CompletenessAndOrdering is a property-based test for call chain completeness
// **Property 3: Call Chain Completeness and Ordering**
// **Validates: Requirements 3.1, 3.2, 4.1, 4.6**
func TestCallChain_Property_CompletenessAndOrdering(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 100, // High limit to not interfere with test
		MaxHotPaths:       100,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Property: For any pprof Sample, the extracted CallChain SHALL:
	// - Contain all frames from the sample's Location list
	// - Maintain the original order (entry point first, leaf last)
	// - Have CategoryBreakdown sum equal to total frame count
	// - Have BoundaryPoints correctly marking category transitions

	f := func(numFrames uint8) bool {
		// Limit to reasonable number of frames
		n := int(numFrames%20) + 1
		if n == 0 {
			return true
		}

		// Create sample with n frames
		funcNames := make([]string, n)
		for i := 0; i < n; i++ {
			// Alternate between different categories
			switch i % 4 {
			case 0:
				funcNames[i] = "github.com/myapp/handler.Func" + string(rune('A'+i))
			case 1:
				funcNames[i] = "encoding/json.Marshal" + string(rune('A'+i))
			case 2:
				funcNames[i] = "runtime.func" + string(rune('A'+i))
			case 3:
				funcNames[i] = "github.com/other/pkg.Func" + string(rune('A'+i))
			}
		}

		sample := createTestSample(funcNames, 1000, classifier)
		p := createTestProfile([]*profile.Sample{sample})

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")
		if len(hotPaths) == 0 {
			t.Log("No hot paths returned")
			return false
		}

		chain := hotPaths[0].Chain

		// Property 1: Contain all frames from the sample's Location list
		if len(chain.Frames) != n {
			t.Logf("Frame count mismatch: got %d, want %d", len(chain.Frames), n)
			return false
		}

		// Property 2: Maintain the original order (entry point first, leaf last)
		// First frame should be the first function name (entry point)
		if chain.Frames[0].FunctionName != funcNames[0] {
			t.Logf("First frame mismatch: got %q, want %q", chain.Frames[0].FunctionName, funcNames[0])
			return false
		}
		// Last frame should be the last function name (leaf)
		if chain.Frames[n-1].FunctionName != funcNames[n-1] {
			t.Logf("Last frame mismatch: got %q, want %q", chain.Frames[n-1].FunctionName, funcNames[n-1])
			return false
		}

		// Property 3: CategoryBreakdown sum equals total frame count
		breakdownSum := GetCategoryBreakdownSum(chain.CategoryBreakdown)
		if breakdownSum != len(chain.Frames) {
			t.Logf("CategoryBreakdown sum mismatch: got %d, want %d", breakdownSum, len(chain.Frames))
			return false
		}

		// Property 4: BoundaryPoints correctly marking category transitions
		// Verify each boundary point marks a category change
		for _, bp := range chain.BoundaryPoints {
			if bp <= 0 || bp >= len(chain.Frames) {
				t.Logf("Invalid boundary point: %d (frames: %d)", bp, len(chain.Frames))
				return false
			}
			if chain.Frames[bp].Category == chain.Frames[bp-1].Category {
				t.Logf("Boundary point %d does not mark category change", bp)
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

// TestHotPathAggregation_Property_Correctness is a property-based test for aggregation
// **Property 4: Hot Path Aggregation Correctness**
// **Validates: Requirements 3.4**
func TestHotPathAggregation_Property_Correctness(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 100,
		MaxHotPaths:       100,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Property: For any set of Samples with identical call paths, aggregation SHALL:
	// - Produce exactly one CallChain
	// - Have TotalValue equal to sum of individual sample values
	// - Have SampleCount equal to number of aggregated samples

	f := func(numDuplicates uint8, value1, value2 uint16) bool {
		// Limit duplicates to reasonable number
		n := int(numDuplicates%10) + 2
		if n < 2 {
			n = 2
		}

		// Create n identical samples with different values
		funcNames := []string{
			"github.com/myapp/handler.ProcessRequest",
			"encoding/json.Marshal",
			"runtime.mallocgc",
		}

		chains := make([]CallChain, n)
		expectedTotalValue := int64(0)
		for i := 0; i < n; i++ {
			sampleValue := int64(value1) + int64(i)*int64(value2%100+1)
			expectedTotalValue += sampleValue

			sample := createTestSample(funcNames, sampleValue, classifier)
			chains[i] = extractor.ExtractCallChain(sample, 0, 10000)
		}

		aggregated := analyzer.AggregateCallChains(chains)

		// Property 1: Produce exactly one CallChain (all paths are identical)
		if len(aggregated) != 1 {
			t.Logf("Expected 1 aggregated chain, got %d", len(aggregated))
			return false
		}

		// Property 2: TotalValue equals sum of individual sample values
		if aggregated[0].TotalValue != expectedTotalValue {
			t.Logf("TotalValue mismatch: got %d, want %d", aggregated[0].TotalValue, expectedTotalValue)
			return false
		}

		// Property 3: SampleCount equals number of aggregated samples
		if aggregated[0].SampleCount != n {
			t.Logf("SampleCount mismatch: got %d, want %d", aggregated[0].SampleCount, n)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestBusinessFrameIdentification_Property is a property-based test for business frame identification
// **Property 5: Business Frame Identification**
// **Validates: Requirements 4.2, 4.4**
func TestBusinessFrameIdentification_Property(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 100,
		MaxHotPaths:       100,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	// Property: For any CallChain, the BusinessFrames list SHALL:
	// - Contain indices of ALL frames where Category == CategoryBusiness
	// - Be sorted in ascending order (entry to leaf)
	// - Be empty if and only if no business frames exist in the chain

	f := func(pattern uint8) bool {
		// Generate a pattern of business/non-business frames
		n := 5
		funcNames := make([]string, n)
		expectedBusinessIndices := make([]int, 0)

		for i := 0; i < n; i++ {
			// Use pattern bits to determine if frame is business
			isBusiness := (pattern>>uint(i))&1 == 1
			if isBusiness {
				funcNames[i] = "github.com/myapp/handler.Func" + string(rune('A'+i))
				expectedBusinessIndices = append(expectedBusinessIndices, i)
			} else {
				funcNames[i] = "runtime.func" + string(rune('A'+i))
			}
		}

		sample := createTestSample(funcNames, 1000, classifier)
		p := createTestProfile([]*profile.Sample{sample})

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")
		if len(hotPaths) == 0 {
			t.Log("No hot paths returned")
			return false
		}

		businessFrames := hotPaths[0].BusinessFrames

		// Property 1: Contain indices of ALL frames where Category == CategoryBusiness
		if len(businessFrames) != len(expectedBusinessIndices) {
			t.Logf("BusinessFrames count mismatch: got %d, want %d", len(businessFrames), len(expectedBusinessIndices))
			return false
		}

		// Property 2: Be sorted in ascending order
		for i := 1; i < len(businessFrames); i++ {
			if businessFrames[i] <= businessFrames[i-1] {
				t.Logf("BusinessFrames not sorted: %v", businessFrames)
				return false
			}
		}

		// Property 3: Be empty if and only if no business frames exist
		hasBusinessFrames := len(expectedBusinessIndices) > 0
		hasBusinessFramesResult := len(businessFrames) > 0
		if hasBusinessFrames != hasBusinessFramesResult {
			t.Logf("Business frame existence mismatch: expected %v, got %v", hasBusinessFrames, hasBusinessFramesResult)
			return false
		}

		// Verify indices match expected
		for i, idx := range businessFrames {
			if idx != expectedBusinessIndices[i] {
				t.Logf("BusinessFrames index mismatch at %d: got %d, want %d", i, idx, expectedBusinessIndices[i])
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

// TestRootCauseIndex tests that RootCauseIndex is correctly set
func TestRootCauseIndex(t *testing.T) {
	config := LocatorConfig{
		ModuleName:        "github.com/myapp",
		MaxCallStackDepth: 10,
		MaxHotPaths:       5,
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)
	analyzer := NewPathAnalyzer(extractor, config)

	t.Run("with business frames", func(t *testing.T) {
		funcNames := []string{
			"github.com/myapp/handler.Entry",     // business - index 0
			"github.com/myapp/handler.Process",   // business - index 1
			"encoding/json.Marshal",              // stdlib - index 2
			"github.com/myapp/handler.DeepLogic", // business - index 3 (deepest)
			"runtime.mallocgc",                   // runtime - index 4
		}
		sample := createTestSample(funcNames, 1000, classifier)
		p := createTestProfile([]*profile.Sample{sample})

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

		assert.Equal(t, 1, len(hotPaths))
		// RootCauseIndex should be the deepest business frame (index 3)
		assert.Equal(t, 3, hotPaths[0].RootCauseIndex)
		assert.Equal(t, []int{0, 1, 3}, hotPaths[0].BusinessFrames)
	})

	t.Run("without business frames", func(t *testing.T) {
		funcNames := []string{
			"encoding/json.Marshal",
			"runtime.mallocgc",
		}
		sample := createTestSample(funcNames, 1000, classifier)
		p := createTestProfile([]*profile.Sample{sample})

		hotPaths := analyzer.AnalyzeHotPaths(p, "cpu")

		assert.Equal(t, 1, len(hotPaths))
		// RootCauseIndex should be -1 when no business frames
		assert.Equal(t, -1, hotPaths[0].RootCauseIndex)
		assert.Equal(t, 0, len(hotPaths[0].BusinessFrames))
	})
}

// TestGetCategoryBreakdownSum tests the helper function
func TestGetCategoryBreakdownSum(t *testing.T) {
	t.Run("empty breakdown", func(t *testing.T) {
		sum := GetCategoryBreakdownSum(nil)
		assert.Equal(t, 0, sum)

		sum = GetCategoryBreakdownSum(map[CodeCategory]int{})
		assert.Equal(t, 0, sum)
	})

	t.Run("with values", func(t *testing.T) {
		breakdown := map[CodeCategory]int{
			CategoryBusiness: 2,
			CategoryStdlib:   3,
			CategoryRuntime:  5,
		}
		sum := GetCategoryBreakdownSum(breakdown)
		assert.Equal(t, 10, sum)
	})
}
