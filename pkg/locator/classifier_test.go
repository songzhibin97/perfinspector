package locator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: problem-locator, Property 2: Code Classification Correctness
// Validates: Requirements 2.1, 2.2, 2.3, 2.4

// TestClassifier_RuntimePackages tests that runtime packages are correctly classified
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.1**
func TestClassifier_RuntimePackages(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any package starting with "runtime" or "runtime/", classification should be CategoryRuntime
	runtimePackages := []string{
		"runtime",
		"runtime/debug",
		"runtime/pprof",
		"runtime/trace",
		"runtime/cgo",
		"runtime/internal/atomic",
	}

	for _, pkg := range runtimePackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryRuntime, category, "Package %s should be classified as runtime", pkg)
	}
}

// TestClassifier_StdlibPackages tests that stdlib packages are correctly classified
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.2**
func TestClassifier_StdlibPackages(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any Go stdlib package, classification should be CategoryStdlib
	stdlibPackages := []string{
		"fmt",
		"net/http",
		"encoding/json",
		"sync",
		"context",
		"io",
		"os",
		"strings",
		"bytes",
		"reflect",
		"time",
		"crypto/tls",
		"database/sql",
		"net/http/httptest",
		"golang.org/x/net",
		"golang.org/x/sync",
	}

	for _, pkg := range stdlibPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryStdlib, category, "Package %s should be classified as stdlib", pkg)
	}
}

// TestClassifier_BusinessPackages tests that business packages are correctly classified
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.4**
func TestClassifier_BusinessPackages(t *testing.T) {
	moduleName := "github.com/mycompany/myapp"
	config := LocatorConfig{
		ModuleName: moduleName,
	}
	classifier := NewClassifier(config)

	// Property: For any package starting with the configured module name, classification should be CategoryBusiness
	businessPackages := []string{
		"github.com/mycompany/myapp",
		"github.com/mycompany/myapp/handler",
		"github.com/mycompany/myapp/internal/service",
		"github.com/mycompany/myapp/pkg/utils",
	}

	for _, pkg := range businessPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryBusiness, category, "Package %s should be classified as business", pkg)
	}
}

// TestClassifier_ThirdPartyPackages tests that third-party packages are correctly classified
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.3**
func TestClassifier_ThirdPartyPackages(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any package with github.com/gitlab.com/gopkg.in (not matching module), classification should be CategoryThirdParty
	thirdPartyPackages := []string{
		"github.com/gin-gonic/gin",
		"github.com/stretchr/testify",
		"github.com/google/pprof",
		"gitlab.com/someorg/somelib",
		"gopkg.in/yaml.v3",
		"go.uber.org/zap",
		"google.golang.org/grpc",
		"k8s.io/client-go",
	}

	for _, pkg := range thirdPartyPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryThirdParty, category, "Package %s should be classified as third_party", pkg)
	}
}

// TestClassifier_UnknownPackages tests that unknown packages are correctly classified
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4**
func TestClassifier_UnknownPackages(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any package that doesn't match any known category, classification should be CategoryUnknown
	// Note: Packages without "/" that aren't stdlib/runtime are now classified as business (per Requirements 1.2)
	unknownPackages := []string{
		"",
		"customdomain.local/somepackage",
	}

	for _, pkg := range unknownPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryUnknown, category, "Package %s should be classified as unknown", pkg)
	}
}

// TestClassifier_Property_ExactlyOneCategory tests that classification always returns exactly one category
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4**
func TestClassifier_Property_ExactlyOneCategory(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/testmodule",
	}
	classifier := NewClassifier(config)

	validCategories := map[CodeCategory]bool{
		CategoryRuntime:    true,
		CategoryStdlib:     true,
		CategoryThirdParty: true,
		CategoryBusiness:   true,
		CategoryUnknown:    true,
	}

	// Property: For any string input, Classify returns exactly one valid CodeCategory
	f := func(packageName string) bool {
		category := classifier.Classify(packageName)
		return validCategories[category]
	}

	config2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, config2); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestClassifier_Property_RuntimePriority tests that runtime classification takes priority
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.1**
func TestClassifier_Property_RuntimePriority(t *testing.T) {
	// Even if module name starts with "runtime", runtime packages should be classified as runtime
	config := LocatorConfig{
		ModuleName: "runtime-app", // Edge case: module name starts with "runtime"
	}
	classifier := NewClassifier(config)

	// runtime package should still be classified as runtime
	assert.Equal(t, CategoryRuntime, classifier.Classify("runtime"))
	assert.Equal(t, CategoryRuntime, classifier.Classify("runtime/debug"))
}

// TestClassifier_Property_BusinessOverThirdParty tests that business code takes priority over third-party
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.3, 2.4**
func TestClassifier_Property_BusinessOverThirdParty(t *testing.T) {
	// User's module on github.com should be classified as business, not third-party
	config := LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	}
	classifier := NewClassifier(config)

	// User's own module should be business, not third-party
	assert.Equal(t, CategoryBusiness, classifier.Classify("github.com/mycompany/myapp"))
	assert.Equal(t, CategoryBusiness, classifier.Classify("github.com/mycompany/myapp/internal"))

	// Other github packages should be third-party
	assert.Equal(t, CategoryThirdParty, classifier.Classify("github.com/othercompany/otherapp"))
}

// TestClassifier_CustomThirdPartyPrefixes tests custom third-party prefixes
// **Property 2: Code Classification Correctness**
// **Validates: Requirements 2.3**
func TestClassifier_CustomThirdPartyPrefixes(t *testing.T) {
	config := LocatorConfig{
		ModuleName:         "myapp",
		ThirdPartyPrefixes: []string{"internal.company.com/", "private.repo/"},
	}
	classifier := NewClassifier(config)

	// Custom prefixes should be classified as third-party
	assert.Equal(t, CategoryThirdParty, classifier.Classify("internal.company.com/shared/lib"))
	assert.Equal(t, CategoryThirdParty, classifier.Classify("private.repo/utils"))
}

// TestDetectModuleName tests module name detection from go.mod
// **Validates: Requirements 2.5**
func TestDetectModuleName(t *testing.T) {
	// Create a temporary directory with a go.mod file
	tempDir, err := os.MkdirTemp("", "gomod-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	goModContent := `module github.com/testorg/testproject

go 1.20

require (
	github.com/stretchr/testify v1.8.4
)
`
	goModPath := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	moduleName, err := DetectModuleName(tempDir)
	require.NoError(t, err)
	assert.Equal(t, "github.com/testorg/testproject", moduleName)
}

// TestDetectModuleName_NotFound tests module name detection when go.mod doesn't exist
// **Validates: Requirements 2.6**
func TestDetectModuleName_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomod-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	_, err = DetectModuleName(tempDir)
	assert.Error(t, err)
}

// TestDetectModuleName_EmptyModule tests module name detection with empty module line
// **Validates: Requirements 2.5**
func TestDetectModuleName_EmptyModule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gomod-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// go.mod without module line
	goModContent := `go 1.20

require (
	github.com/stretchr/testify v1.8.4
)
`
	goModPath := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	_, err = DetectModuleName(tempDir)
	assert.Error(t, err)
}

// TestClassifier_NoModuleName tests classification when no module name is configured
// **Validates: Requirements 2.6**
func TestClassifier_NoModuleName(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "", // No module name
	}
	classifier := NewClassifier(config)

	// Without module name, github packages should be third-party
	assert.Equal(t, CategoryThirdParty, classifier.Classify("github.com/someorg/someapp"))

	// Packages without "/" that aren't stdlib/runtime are now classified as business (per Requirements 1.2)
	assert.Equal(t, CategoryBusiness, classifier.Classify("someunknownpackage"))

	// Runtime and stdlib should still work
	assert.Equal(t, CategoryRuntime, classifier.Classify("runtime"))
	assert.Equal(t, CategoryStdlib, classifier.Classify("fmt"))
}

// ============================================================================
// Feature: report-improvements, Property Tests for Classifier Improvements
// ============================================================================

// TestClassifier_Property_MainPackageIsBusiness tests that main package functions are classified as business
// Feature: report-improvements, Property 1: Main 包函数分类为 Business
// **Validates: Requirements 1.1, 1.5**
func TestClassifier_Property_MainPackageIsBusiness(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any function name starting with "main." or equal to "main",
	// Classifier.Classify() SHALL return CategoryBusiness
	mainPackages := []string{
		"main",
		"main.init",
		"main.main",
		"main.handleRequest",
		"main.processData",
		"main.(*Server).Start",
		"main.(*Handler).ServeHTTP",
	}

	for _, pkg := range mainPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryBusiness, category,
			"Package %s should be classified as business (main package)", pkg)
	}

	// Property test with quick.Check
	f := func(suffix string) bool {
		// Generate main package names
		if len(suffix) == 0 {
			return classifier.Classify("main") == CategoryBusiness
		}
		// Filter out invalid suffixes (containing control characters)
		for _, r := range suffix {
			if r < 32 || r > 126 {
				return true // Skip invalid inputs
			}
		}
		mainPkg := "main." + suffix
		return classifier.Classify(mainPkg) == CategoryBusiness
	}

	config2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, config2); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestClassifier_Property_NoSlashNonStdlibIsBusiness tests that packages without "/" that aren't stdlib are business
// Feature: report-improvements, Property 2: 无斜杠非标准库函数分类为 Business
// **Validates: Requirements 1.2**
func TestClassifier_Property_NoSlashNonStdlibIsBusiness(t *testing.T) {
	config := LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	}
	classifier := NewClassifier(config)

	// Property: For any function name without "/" that doesn't start with runtime/sync/syscall etc,
	// Classifier.Classify() SHALL return CategoryBusiness
	noSlashBusinessPackages := []string{
		"mypackage",
		"handler",
		"service",
		"utils",
		"models",
		"myapp",
		"custompackage",
		"localmodule",
	}

	for _, pkg := range noSlashBusinessPackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryBusiness, category,
			"Package %s (no slash, non-stdlib) should be classified as business", pkg)
	}

	// Verify stdlib packages without "/" are still classified correctly
	stdlibNoSlash := []string{
		"fmt",
		"sync",
		"runtime",
		"syscall",
		"reflect",
		"strings",
		"bytes",
		"errors",
		"context",
		"io",
		"os",
		"time",
	}

	for _, pkg := range stdlibNoSlash {
		category := classifier.Classify(pkg)
		assert.NotEqual(t, CategoryBusiness, category,
			"Stdlib package %s should NOT be classified as business", pkg)
	}
}

// TestClassifier_Property_ModuleNameIsBusiness tests that project module functions are classified as business
// Feature: report-improvements, Property 3: 项目模块函数分类为 Business
// **Validates: Requirements 1.3**
func TestClassifier_Property_ModuleNameIsBusiness(t *testing.T) {
	moduleName := "github.com/mycompany/myapp"
	config := LocatorConfig{
		ModuleName: moduleName,
	}
	classifier := NewClassifier(config)

	// Property: For any Classifier with moduleName configured,
	// when function name starts with moduleName, Classify() SHALL return CategoryBusiness
	modulePackages := []string{
		"github.com/mycompany/myapp",
		"github.com/mycompany/myapp/handler",
		"github.com/mycompany/myapp/internal/service",
		"github.com/mycompany/myapp/pkg/utils",
		"github.com/mycompany/myapp/cmd/server",
		"github.com/mycompany/myapp/api/v1",
	}

	for _, pkg := range modulePackages {
		category := classifier.Classify(pkg)
		assert.Equal(t, CategoryBusiness, category,
			"Package %s (matches module name) should be classified as business", pkg)
	}

	// Property test with quick.Check - generate random subpaths
	f := func(subpath string) bool {
		// Filter out invalid characters
		for _, r := range subpath {
			if r < 32 || r > 126 || r == ' ' {
				return true // Skip invalid inputs
			}
		}
		if len(subpath) == 0 {
			return classifier.Classify(moduleName) == CategoryBusiness
		}
		fullPath := moduleName + "/" + subpath
		return classifier.Classify(fullPath) == CategoryBusiness
	}

	config2 := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, config2); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestCallChain_Property_HasBusinessCodeWithBusinessFrame tests that CallChain with business frames returns true
// Feature: report-improvements, Property 4: 包含 Business 帧的 HotPath 不显示警告
// **Validates: Requirements 1.4**
func TestCallChain_Property_HasBusinessCodeWithBusinessFrame(t *testing.T) {
	// Property: For any CallChain where at least one Frame has Category == CategoryBusiness,
	// HasBusinessCode() SHALL return true

	// Test case 1: Single business frame
	chain1 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "main.handleRequest", Category: CategoryBusiness},
		},
	}
	assert.True(t, chain1.HasBusinessCode(),
		"CallChain with single business frame should return true")

	// Test case 2: Business frame among runtime frames
	chain2 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "runtime.goexit", Category: CategoryRuntime},
			{FunctionName: "runtime.main", Category: CategoryRuntime},
			{FunctionName: "main.main", Category: CategoryBusiness},
			{FunctionName: "main.handleRequest", Category: CategoryBusiness},
		},
	}
	assert.True(t, chain2.HasBusinessCode(),
		"CallChain with business frames among runtime frames should return true")

	// Test case 3: Business frame at the end
	chain3 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "runtime.goexit", Category: CategoryRuntime},
			{FunctionName: "net/http.(*Server).Serve", Category: CategoryStdlib},
			{FunctionName: "github.com/gin-gonic/gin.(*Engine).Run", Category: CategoryThirdParty},
			{FunctionName: "main.handler", Category: CategoryBusiness},
		},
	}
	assert.True(t, chain3.HasBusinessCode(),
		"CallChain with business frame at end should return true")

	// Test case 4: No business frames - should return false
	chain4 := CallChain{
		Frames: []StackFrame{
			{FunctionName: "runtime.goexit", Category: CategoryRuntime},
			{FunctionName: "runtime.main", Category: CategoryRuntime},
			{FunctionName: "sync.(*WaitGroup).Wait", Category: CategoryStdlib},
		},
	}
	assert.False(t, chain4.HasBusinessCode(),
		"CallChain without business frames should return false")

	// Test case 5: Empty chain - should return false
	chain5 := CallChain{
		Frames: []StackFrame{},
	}
	assert.False(t, chain5.HasBusinessCode(),
		"Empty CallChain should return false")

	// Property test with quick.Check
	// Generate random CallChains and verify the property
	f := func(numFrames uint8) bool {
		// Limit to reasonable size
		n := int(numFrames % 20)
		if n == 0 {
			// Empty chain should return false
			chain := CallChain{Frames: []StackFrame{}}
			return !chain.HasBusinessCode()
		}

		// Generate frames with random categories
		categories := []CodeCategory{
			CategoryRuntime,
			CategoryStdlib,
			CategoryThirdParty,
			CategoryBusiness,
			CategoryUnknown,
		}

		frames := make([]StackFrame, n)
		hasBusiness := false
		for i := 0; i < n; i++ {
			cat := categories[i%len(categories)]
			if cat == CategoryBusiness {
				hasBusiness = true
			}
			frames[i] = StackFrame{
				FunctionName: "test.func" + itoa(int64(i)),
				Category:     cat,
			}
		}

		chain := CallChain{Frames: frames}
		return chain.HasBusinessCode() == hasBusiness
	}

	config := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestCallChain_Property_HasBusinessCodeIntegration tests HasBusinessCode with classifier integration
// Feature: report-improvements, Property 4: 包含 Business 帧的 HotPath 不显示警告
// **Validates: Requirements 1.4**
func TestCallChain_Property_HasBusinessCodeIntegration(t *testing.T) {
	// Test that the improved classifier correctly identifies business code
	// and HasBusinessCode returns true for chains with main package functions

	classifier := NewClassifier(LocatorConfig{
		ModuleName: "github.com/mycompany/myapp",
	})

	// Helper to extract package name from function name
	extractPackage := func(fn string) string {
		// Handle method receivers like "sync.(*WaitGroup).Wait" -> "sync"
		// or "github.com/pkg/errors.New" -> "github.com/pkg/errors"
		// or "main.handleRequest" -> "main"

		// Find the last component that looks like a type or function
		// For "sync.(*WaitGroup).Wait", we want "sync"
		// For "main.main", we want "main"

		// Remove method receiver notation
		fn = strings.ReplaceAll(fn, "(*", ".")
		fn = strings.ReplaceAll(fn, ")", "")

		// Split by dots and find the package path
		parts := strings.Split(fn, ".")
		if len(parts) <= 1 {
			return fn
		}

		// For paths with "/", find the last "/" and take everything before the function
		if strings.Contains(fn, "/") {
			lastSlash := strings.LastIndex(fn, "/")
			afterSlash := fn[lastSlash+1:]
			beforeSlash := fn[:lastSlash]

			// afterSlash might be "errors.New" or "handler.ServeHTTP"
			dotIdx := strings.Index(afterSlash, ".")
			if dotIdx > 0 {
				return beforeSlash + "/" + afterSlash[:dotIdx]
			}
			return fn
		}

		// For simple packages like "sync.WaitGroup.Wait" or "main.main"
		return parts[0]
	}

	// Create frames using the classifier
	testCases := []struct {
		name        string
		functions   []string
		expectTrue  bool
		description string
	}{
		{
			name:        "main package functions",
			functions:   []string{"runtime.goexit", "runtime.main", "main.main", "main.handleRequest"},
			expectTrue:  true,
			description: "Chain with main package should have business code",
		},
		{
			name:        "module functions",
			functions:   []string{"runtime.goexit", "net/http.ListenAndServe", "github.com/mycompany/myapp/handler.ServeHTTP"},
			expectTrue:  true,
			description: "Chain with module functions should have business code",
		},
		{
			name:        "local package without slash",
			functions:   []string{"runtime.goexit", "myhandler.Process"},
			expectTrue:  true,
			description: "Chain with local package (no slash) should have business code",
		},
		{
			name:        "only runtime and stdlib",
			functions:   []string{"runtime.goexit", "runtime.main", "sync.WaitGroup.Wait", "time.Sleep"},
			expectTrue:  false,
			description: "Chain with only runtime/stdlib should not have business code",
		},
		{
			name:        "third party only",
			functions:   []string{"runtime.goexit", "github.com/gin-gonic/gin.Engine.Run"},
			expectTrue:  false,
			description: "Chain with only third-party should not have business code",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			frames := make([]StackFrame, len(tc.functions))
			for i, fn := range tc.functions {
				pkgName := extractPackage(fn)
				frames[i] = StackFrame{
					FunctionName: fn,
					PackageName:  pkgName,
					Category:     classifier.Classify(pkgName),
				}
			}

			chain := CallChain{Frames: frames}
			result := chain.HasBusinessCode()

			assert.Equal(t, tc.expectTrue, result, tc.description)
		})
	}
}
