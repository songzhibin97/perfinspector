package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampHandling(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "perfinspector-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建真实 pprof 文件（带元数据）
	testFile := filepath.Join(tempDir, "cpu.pprof")
	expectedTime := time.Date(2023, 11, 15, 14, 30, 22, 0, time.UTC)
	createValidProfile(t, testFile, expectedTime)

	// 验证时间戳来源
	p, err := parser.LoadProfile(testFile)
	require.NoError(t, err)

	timestamp := parser.GetProfileTime(p)
	assert.Equal(t, expectedTime.Format(time.RFC3339), timestamp.Format(time.RFC3339))
}

func TestGroupProfiles(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "perfinspector-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建多个测试文件
	cpuFile1 := filepath.Join(tempDir, "cpu1.pprof")
	cpuFile2 := filepath.Join(tempDir, "cpu2.pprof")
	heapFile := filepath.Join(tempDir, "heap.pprof")

	time1 := time.Date(2023, 11, 15, 14, 30, 0, 0, time.UTC)
	time2 := time.Date(2023, 11, 15, 14, 35, 0, 0, time.UTC)
	time3 := time.Date(2023, 11, 15, 14, 40, 0, 0, time.UTC)

	createCPUProfile(t, cpuFile1, time1)
	createCPUProfile(t, cpuFile2, time2)
	createHeapProfile(t, heapFile, time3)

	// 测试分组
	paths := []string{cpuFile1, cpuFile2, heapFile}
	groups, err := GroupProfiles(paths)
	require.NoError(t, err)

	// 验证分组结果
	assert.Len(t, groups, 2) // cpu 和 heap 两组

	// 查找 cpu 组
	var cpuGroup *ProfileGroup
	var heapGroup *ProfileGroup
	for i := range groups {
		if groups[i].Type == "cpu" {
			cpuGroup = &groups[i]
		}
		if groups[i].Type == "heap" {
			heapGroup = &groups[i]
		}
	}

	require.NotNil(t, cpuGroup)
	require.NotNil(t, heapGroup)

	assert.Len(t, cpuGroup.Files, 2)
	assert.Len(t, heapGroup.Files, 1)

	// 验证 CPU 文件按时间排序
	assert.True(t, cpuGroup.Files[0].Time.Before(cpuGroup.Files[1].Time))
}

func TestDetectProfileType(t *testing.T) {
	tests := []struct {
		name     string
		profile  *profile.Profile
		expected string
	}{
		{
			name:     "nil profile",
			profile:  nil,
			expected: "unknown",
		},
		{
			name: "cpu profile by sample type",
			profile: &profile.Profile{
				SampleType: []*profile.ValueType{
					{Type: "samples", Unit: "count"},
					{Type: "cpu", Unit: "nanoseconds"},
				},
			},
			expected: "cpu",
		},
		{
			name: "heap profile",
			profile: &profile.Profile{
				SampleType: []*profile.ValueType{
					{Type: "alloc_objects", Unit: "count"},
					{Type: "alloc_space", Unit: "bytes"},
				},
			},
			expected: "heap",
		},
		{
			name: "goroutine profile",
			profile: &profile.Profile{
				SampleType: []*profile.ValueType{
					{Type: "goroutine", Unit: "count"},
				},
			},
			expected: "goroutine",
		},
		{
			name: "cpu profile by duration",
			profile: &profile.Profile{
				DurationNanos: 1000000000,
			},
			expected: "cpu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectProfileType(tt.profile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// createValidProfile 创建带有指定时间戳的有效 pprof 文件
func createValidProfile(t *testing.T, path string, timestamp time.Time) {
	p := &profile.Profile{
		TimeNanos:     timestamp.UnixNano(),
		DurationNanos: 1000000000, // 1秒持续时间
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
		},
		Sample: []*profile.Sample{
			{Value: []int64{100, 1000000}},
		},
	}

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, p.Write(f))
}

// createCPUProfile 创建 CPU profile
func createCPUProfile(t *testing.T, path string, timestamp time.Time) {
	p := &profile.Profile{
		TimeNanos:     timestamp.UnixNano(),
		DurationNanos: 1000000000,
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
		},
		Sample: []*profile.Sample{
			{Value: []int64{100, 1000000}},
		},
	}

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, p.Write(f))
}

// createHeapProfile 创建 Heap profile
func createHeapProfile(t *testing.T, path string, timestamp time.Time) {
	p := &profile.Profile{
		TimeNanos: timestamp.UnixNano(),
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"},
		},
		Sample: []*profile.Sample{
			{Value: []int64{10, 1024, 5, 512}},
		},
	}

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, p.Write(f))
}
