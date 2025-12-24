package locator

import (
	"testing"
	"testing/quick"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
)

// Feature: problem-locator, Property 1: Stack Frame Extraction Completeness
// Validates: Requirements 1.1, 1.2, 1.3, 1.4

// TestExtractPackageName tests package name extraction from function names
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.3**
func TestExtractPackageName(t *testing.T) {
	testCases := []struct {
		functionName string
		expected     string
	}{
		// Simple cases
		{"runtime.mallocgc", "runtime"},
		{"main.main", "main"},
		{"fmt.Println", "fmt"},

		// With path
		{"github.com/user/repo/pkg.Function", "github.com/user/repo/pkg"},
		{"github.com/user/repo/pkg/sub.Function", "github.com/user/repo/pkg/sub"},

		// With method receiver
		{"github.com/user/repo/pkg.(*Type).Method", "github.com/user/repo/pkg"},
		{"github.com/user/repo/pkg.Type.Method", "github.com/user/repo/pkg"},
		{"net/http.(*Server).Serve", "net/http"},

		// Edge cases
		{"", ""},
		{"singleword", "singleword"}, // No dot, returns as-is
	}

	for _, tc := range testCases {
		t.Run(tc.functionName, func(t *testing.T) {
			result := ExtractPackageName(tc.functionName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractShortName tests short name extraction from function names
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1**
func TestExtractShortName(t *testing.T) {
	testCases := []struct {
		functionName string
		expected     string
	}{
		// Simple cases
		{"runtime.mallocgc", "mallocgc"},
		{"main.main", "main"},
		{"fmt.Println", "Println"},

		// With path
		{"github.com/user/repo/pkg.Function", "Function"},
		{"github.com/user/repo/pkg/sub.Function", "Function"},

		// With method receiver
		{"github.com/user/repo/pkg.(*Type).Method", "(*Type).Method"},
		{"github.com/user/repo/pkg.Type.Method", "Type.Method"},
		{"net/http.(*Server).Serve", "(*Server).Serve"},

		// Edge cases
		{"", ""},
		{"singleword", "singleword"}, // No dot, returns as-is
	}

	for _, tc := range testCases {
		t.Run(tc.functionName, func(t *testing.T) {
			result := ExtractShortName(tc.functionName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractStackFrame_Complete tests that stack frame extraction is complete
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
func TestExtractStackFrame_Complete(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	// Create a mock pprof Location and Line
	fn := &profile.Function{
		ID:       1,
		Name:     "github.com/myapp/handler.ProcessRequest",
		Filename: "/home/user/myapp/handler/request.go",
	}

	line := &profile.Line{
		Function: fn,
		Line:     42,
	}

	loc := &profile.Location{
		ID:   1,
		Line: []profile.Line{*line},
	}

	frame := extractor.ExtractStackFrame(loc, line)

	// Property: FunctionName equals the original Function.Name
	assert.Equal(t, fn.Name, frame.FunctionName)

	// Property: FilePath equals the original Function.Filename
	assert.Equal(t, fn.Filename, frame.FilePath)

	// Property: LineNumber equals the original Line.Line
	assert.Equal(t, line.Line, frame.LineNumber)

	// Property: PackageName is correctly extracted from FunctionName
	assert.Equal(t, "github.com/myapp/handler", frame.PackageName)

	// Property: ShortName is correctly extracted
	assert.Equal(t, "ProcessRequest", frame.ShortName)

	// Property: Category is correctly classified
	assert.Equal(t, CategoryBusiness, frame.Category)
}

// TestExtractStackFrame_MissingInfo tests fallback behavior for missing info
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.4**
func TestExtractStackFrame_MissingInfo(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	t.Run("nil line", func(t *testing.T) {
		loc := &profile.Location{ID: 1}
		frame := extractor.ExtractStackFrame(loc, nil)

		// Should return unknown values
		assert.Equal(t, "unknown", frame.FunctionName)
		assert.Equal(t, "unknown", frame.FilePath)
		assert.Equal(t, int64(0), frame.LineNumber)
	})

	t.Run("nil function", func(t *testing.T) {
		line := &profile.Line{
			Function: nil,
			Line:     10,
		}
		loc := &profile.Location{
			ID:   1,
			Line: []profile.Line{*line},
		}
		frame := extractor.ExtractStackFrame(loc, line)

		// Should return unknown values
		assert.Equal(t, "unknown", frame.FunctionName)
		assert.Equal(t, "unknown", frame.FilePath)
	})

	t.Run("empty filename", func(t *testing.T) {
		fn := &profile.Function{
			ID:       1,
			Name:     "runtime.mallocgc",
			Filename: "", // Empty filename
		}
		line := &profile.Line{
			Function: fn,
			Line:     100,
		}
		loc := &profile.Location{
			ID:   1,
			Line: []profile.Line{*line},
		}
		frame := extractor.ExtractStackFrame(loc, line)

		// FunctionName should be set
		assert.Equal(t, "runtime.mallocgc", frame.FunctionName)
		// FilePath should fallback to "unknown"
		assert.Equal(t, "unknown", frame.FilePath)
		// LineNumber should be set
		assert.Equal(t, int64(100), frame.LineNumber)
	})

	t.Run("zero line number", func(t *testing.T) {
		fn := &profile.Function{
			ID:       1,
			Name:     "runtime.mallocgc",
			Filename: "/usr/local/go/src/runtime/malloc.go",
		}
		line := &profile.Line{
			Function: fn,
			Line:     0, // Zero line number
		}
		loc := &profile.Location{
			ID:   1,
			Line: []profile.Line{*line},
		}
		frame := extractor.ExtractStackFrame(loc, line)

		// LineNumber should be 0
		assert.Equal(t, int64(0), frame.LineNumber)
	})
}

// TestExtractCallChain tests call chain extraction from samples
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1, 1.2, 1.3, 3.2**
func TestExtractCallChain(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	// Create a mock sample with multiple locations
	// pprof stores locations from leaf to root, so we create them in that order
	fn1 := &profile.Function{ID: 1, Name: "runtime.mallocgc", Filename: "runtime/malloc.go"}
	fn2 := &profile.Function{ID: 2, Name: "encoding/json.Marshal", Filename: "encoding/json/encode.go"}
	fn3 := &profile.Function{ID: 3, Name: "github.com/myapp/handler.ProcessRequest", Filename: "handler/request.go"}

	loc1 := &profile.Location{ID: 1, Line: []profile.Line{{Function: fn1, Line: 100}}}
	loc2 := &profile.Location{ID: 2, Line: []profile.Line{{Function: fn2, Line: 200}}}
	loc3 := &profile.Location{ID: 3, Line: []profile.Line{{Function: fn3, Line: 42}}}

	sample := &profile.Sample{
		Location: []*profile.Location{loc1, loc2, loc3}, // leaf to root
		Value:    []int64{1000},
	}

	chain := extractor.ExtractCallChain(sample, 0, 10000)

	// Property: Contain all frames from the sample's Location list
	assert.Equal(t, 3, len(chain.Frames))

	// Property: Maintain the original order (entry point first, leaf last)
	// After reversal: fn3 (business) -> fn2 (stdlib) -> fn1 (runtime)
	assert.Equal(t, "github.com/myapp/handler.ProcessRequest", chain.Frames[0].FunctionName)
	assert.Equal(t, "encoding/json.Marshal", chain.Frames[1].FunctionName)
	assert.Equal(t, "runtime.mallocgc", chain.Frames[2].FunctionName)

	// Property: TotalValue is set correctly
	assert.Equal(t, int64(1000), chain.TotalValue)

	// Property: TotalPct is calculated correctly
	assert.Equal(t, 10.0, chain.TotalPct) // 1000/10000 * 100 = 10%

	// Property: SampleCount is 1
	assert.Equal(t, 1, chain.SampleCount)

	// Property: CategoryBreakdown sum equals total frame count
	totalBreakdown := 0
	for _, count := range chain.CategoryBreakdown {
		totalBreakdown += count
	}
	assert.Equal(t, len(chain.Frames), totalBreakdown)

	// Property: BoundaryPoints correctly marking category transitions
	// Frames: business -> stdlib -> runtime (2 transitions)
	assert.Equal(t, 2, len(chain.BoundaryPoints))
	assert.Contains(t, chain.BoundaryPoints, 1) // business -> stdlib
	assert.Contains(t, chain.BoundaryPoints, 2) // stdlib -> runtime
}

// TestExtractCallChain_EmptySample tests call chain extraction with empty sample
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.4**
func TestExtractCallChain_EmptySample(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	t.Run("nil sample", func(t *testing.T) {
		chain := extractor.ExtractCallChain(nil, 0, 10000)
		assert.Equal(t, 0, len(chain.Frames))
		assert.Equal(t, int64(0), chain.TotalValue)
	})

	t.Run("empty locations", func(t *testing.T) {
		sample := &profile.Sample{
			Location: []*profile.Location{},
			Value:    []int64{1000},
		}
		chain := extractor.ExtractCallChain(sample, 0, 10000)
		assert.Equal(t, 0, len(chain.Frames))
		assert.Equal(t, int64(1000), chain.TotalValue)
	})
}

// TestExtractCallChain_InlinedFunctions tests call chain extraction with inlined functions
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1, 1.2, 1.3**
func TestExtractCallChain_InlinedFunctions(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	// Create a location with multiple lines (inlined functions)
	fn1 := &profile.Function{ID: 1, Name: "runtime.mallocgc", Filename: "runtime/malloc.go"}
	fn2 := &profile.Function{ID: 2, Name: "runtime.newobject", Filename: "runtime/malloc.go"}

	// Location with inlined functions (multiple lines)
	loc := &profile.Location{
		ID: 1,
		Line: []profile.Line{
			{Function: fn1, Line: 100}, // innermost (leaf)
			{Function: fn2, Line: 50},  // outer (inlined into)
		},
	}

	sample := &profile.Sample{
		Location: []*profile.Location{loc},
		Value:    []int64{1000},
	}

	chain := extractor.ExtractCallChain(sample, 0, 10000)

	// Both inlined functions should be extracted
	assert.Equal(t, 2, len(chain.Frames))

	// Order should be from outer to inner (entry to leaf)
	assert.Equal(t, "runtime.newobject", chain.Frames[0].FunctionName)
	assert.Equal(t, "runtime.mallocgc", chain.Frames[1].FunctionName)
}

// TestExtractStackFrame_Property_Completeness is a property-based test for stack frame extraction
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1, 1.2, 1.3, 1.4**
func TestExtractStackFrame_Property_Completeness(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	// Property: For any valid pprof profile with Location and Line information,
	// extracting a StackFrame SHALL produce a result where:
	// - FunctionName equals the original Function.Name
	// - FilePath equals the original Function.Filename (or "unknown" if empty)
	// - LineNumber equals the original Line.Line (or 0 if unavailable)
	// - PackageName is correctly extracted from FunctionName

	f := func(funcName, fileName string, lineNum int64) bool {
		// Skip empty function names as they're edge cases handled separately
		if funcName == "" {
			return true
		}

		// Ensure lineNum is non-negative for valid test cases
		if lineNum < 0 {
			lineNum = 0
		}

		fn := &profile.Function{
			ID:       1,
			Name:     funcName,
			Filename: fileName,
		}

		line := &profile.Line{
			Function: fn,
			Line:     lineNum,
		}

		loc := &profile.Location{
			ID:   1,
			Line: []profile.Line{*line},
		}

		frame := extractor.ExtractStackFrame(loc, line)

		// Property 1: FunctionName equals the original Function.Name
		if frame.FunctionName != funcName {
			t.Logf("FunctionName mismatch: got %q, want %q", frame.FunctionName, funcName)
			return false
		}

		// Property 2: FilePath equals the original Function.Filename (or "unknown" if empty)
		expectedFilePath := fileName
		if fileName == "" {
			expectedFilePath = "unknown"
		}
		if frame.FilePath != expectedFilePath {
			t.Logf("FilePath mismatch: got %q, want %q", frame.FilePath, expectedFilePath)
			return false
		}

		// Property 3: LineNumber equals the original Line.Line
		if frame.LineNumber != lineNum {
			t.Logf("LineNumber mismatch: got %d, want %d", frame.LineNumber, lineNum)
			return false
		}

		// Property 4: PackageName is correctly extracted from FunctionName
		expectedPkg := ExtractPackageName(funcName)
		if frame.PackageName != expectedPkg {
			t.Logf("PackageName mismatch: got %q, want %q", frame.PackageName, expectedPkg)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestExtractCallChain_Property_FrameCount is a property-based test for call chain frame count
// **Property 1: Stack Frame Extraction Completeness**
// **Validates: Requirements 1.1, 3.2**
func TestExtractCallChain_Property_FrameCount(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)
	extractor := NewExtractor(classifier)

	// Property: CategoryBreakdown sum equals total frame count
	f := func(numLocations uint8) bool {
		// Limit to reasonable number of locations
		n := int(numLocations % 20)
		if n == 0 {
			return true
		}

		locations := make([]*profile.Location, n)
		for i := 0; i < n; i++ {
			fn := &profile.Function{
				ID:       uint64(i + 1),
				Name:     "pkg.Function" + string(rune('A'+i)),
				Filename: "file.go",
			}
			locations[i] = &profile.Location{
				ID:   uint64(i + 1),
				Line: []profile.Line{{Function: fn, Line: int64(i + 1)}},
			}
		}

		sample := &profile.Sample{
			Location: locations,
			Value:    []int64{1000},
		}

		chain := extractor.ExtractCallChain(sample, 0, 10000)

		// Property: Number of frames equals number of locations (with single line each)
		if len(chain.Frames) != n {
			t.Logf("Frame count mismatch: got %d, want %d", len(chain.Frames), n)
			return false
		}

		// Property: CategoryBreakdown sum equals total frame count
		totalBreakdown := 0
		for _, count := range chain.CategoryBreakdown {
			totalBreakdown += count
		}
		if totalBreakdown != len(chain.Frames) {
			t.Logf("CategoryBreakdown sum mismatch: got %d, want %d", totalBreakdown, len(chain.Frames))
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
