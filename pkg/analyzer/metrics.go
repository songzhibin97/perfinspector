package analyzer

import (
	"sort"
	"time"

	"github.com/google/pprof/profile"
)

// ProfileMetrics 单个 profile 的性能指标
type ProfileMetrics struct {
	// 通用指标
	TotalSamples int64
	TotalValue   int64
	Duration     time.Duration
	NumLocations int
	NumFunctions int

	// CPU 指标
	CPUTime time.Duration

	// Heap 指标
	AllocObjects int64
	AllocSpace   int64 // bytes
	InuseObjects int64
	InuseSpace   int64 // bytes

	// Goroutine 指标
	GoroutineCount int64

	// Top 函数
	TopFunctions []FunctionStat
}

// FunctionStat 函数统计
type FunctionStat struct {
	Name    string
	Flat    int64   // 自身消耗
	FlatPct float64 // 自身消耗百分比
	Cum     int64   // 累计消耗（包含调用的函数）
	CumPct  float64 // 累计消耗百分比
}

// ExtractMetrics 从 profile 中提取性能指标
func ExtractMetrics(p *profile.Profile, profileType string) *ProfileMetrics {
	if p == nil {
		return nil
	}

	metrics := &ProfileMetrics{
		NumLocations: len(p.Location),
		NumFunctions: len(p.Function),
	}

	if p.DurationNanos > 0 {
		metrics.Duration = time.Duration(p.DurationNanos)
	}

	// 计算总样本数和总值
	for _, sample := range p.Sample {
		metrics.TotalSamples++
		if len(sample.Value) > 0 {
			metrics.TotalValue += sample.Value[0]
		}
	}

	// 根据类型提取特定指标
	switch profileType {
	case "cpu":
		metrics.CPUTime = extractCPUTime(p)
		metrics.TopFunctions = extractTopFunctions(p, 10, 1) // CPU 时间在 index 1
	case "heap":
		metrics.AllocObjects, metrics.AllocSpace, metrics.InuseObjects, metrics.InuseSpace = extractHeapMetrics(p)
		metrics.TopFunctions = extractTopFunctions(p, 10, 1) // alloc_space 在 index 1
	case "goroutine":
		metrics.GoroutineCount = extractGoroutineCount(p)
		metrics.TopFunctions = extractTopFunctions(p, 10, 0)
	default:
		metrics.TopFunctions = extractTopFunctions(p, 10, 0)
	}

	return metrics
}

// extractCPUTime 提取 CPU 时间
func extractCPUTime(p *profile.Profile) time.Duration {
	var totalNanos int64

	// 查找 CPU 时间的 sample type index
	cpuIndex := -1
	for i, st := range p.SampleType {
		if st.Type == "cpu" && st.Unit == "nanoseconds" {
			cpuIndex = i
			break
		}
	}

	if cpuIndex == -1 && len(p.SampleType) > 1 {
		cpuIndex = 1 // 默认第二列是 CPU 时间
	}

	if cpuIndex >= 0 {
		for _, sample := range p.Sample {
			if cpuIndex < len(sample.Value) {
				totalNanos += sample.Value[cpuIndex]
			}
		}
	}

	return time.Duration(totalNanos)
}

// extractHeapMetrics 提取堆内存指标
func extractHeapMetrics(p *profile.Profile) (allocObjects, allocSpace, inuseObjects, inuseSpace int64) {
	// 查找各指标的 index
	indices := make(map[string]int)
	for i, st := range p.SampleType {
		indices[st.Type] = i
	}

	for _, sample := range p.Sample {
		if idx, ok := indices["alloc_objects"]; ok && idx < len(sample.Value) {
			allocObjects += sample.Value[idx]
		}
		if idx, ok := indices["alloc_space"]; ok && idx < len(sample.Value) {
			allocSpace += sample.Value[idx]
		}
		if idx, ok := indices["inuse_objects"]; ok && idx < len(sample.Value) {
			inuseObjects += sample.Value[idx]
		}
		if idx, ok := indices["inuse_space"]; ok && idx < len(sample.Value) {
			inuseSpace += sample.Value[idx]
		}
	}

	return
}

// extractGoroutineCount 提取 goroutine 数量
func extractGoroutineCount(p *profile.Profile) int64 {
	var count int64
	for _, sample := range p.Sample {
		if len(sample.Value) > 0 {
			count += sample.Value[0]
		}
	}
	return count
}

// extractTopFunctions 提取 Top N 函数
func extractTopFunctions(p *profile.Profile, n int, valueIndex int) []FunctionStat {
	if p == nil || len(p.Sample) == 0 {
		return nil
	}

	// 计算每个函数的 flat 和 cum 值
	flatMap := make(map[uint64]int64) // function ID -> flat value
	cumMap := make(map[uint64]int64)  // function ID -> cum value
	funcMap := make(map[uint64]*profile.Function)

	var totalValue int64

	for _, sample := range p.Sample {
		if len(sample.Value) <= valueIndex {
			continue
		}
		value := sample.Value[valueIndex]
		totalValue += value

		// 遍历调用栈
		for i, loc := range sample.Location {
			if loc == nil {
				continue
			}
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				funcID := line.Function.ID
				funcMap[funcID] = line.Function

				// Cum: 所有出现的位置都计入
				cumMap[funcID] += value

				// Flat: 只有栈顶（第一个位置）计入
				if i == 0 {
					flatMap[funcID] += value
				}
			}
		}
	}

	// 转换为切片并排序
	var stats []FunctionStat
	for funcID, flat := range flatMap {
		fn := funcMap[funcID]
		if fn == nil {
			continue
		}

		name := fn.Name
		if name == "" {
			name = "<unknown>"
		}

		cum := cumMap[funcID]

		var flatPct, cumPct float64
		if totalValue > 0 {
			flatPct = float64(flat) / float64(totalValue) * 100
			cumPct = float64(cum) / float64(totalValue) * 100
		}

		stats = append(stats, FunctionStat{
			Name:    name,
			Flat:    flat,
			FlatPct: flatPct,
			Cum:     cum,
			CumPct:  cumPct,
		})
	}

	// 按 flat 值降序排序
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Flat > stats[j].Flat
	})

	// 取 Top N
	if len(stats) > n {
		stats = stats[:n]
	}

	return stats
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/GB) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/MB) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/KB) + " KB"
	default:
		return formatInt(bytes) + " B"
	}
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	}
	return formatFloatPrecision(f, 2)
}

func formatInt(i int64) string {
	s := ""
	for i > 0 {
		if s != "" {
			s = "," + s
		}
		if i >= 1000 {
			s = formatThreeDigits(i%1000) + s
			i /= 1000
		} else {
			s = itoa(i) + s
			break
		}
	}
	if s == "" {
		return "0"
	}
	return s
}

func formatThreeDigits(n int64) string {
	if n < 10 {
		return "00" + itoa(n)
	}
	if n < 100 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func formatFloatPrecision(f float64, precision int) string {
	// 简单实现
	intPart := int64(f)
	fracPart := f - float64(intPart)

	result := itoa(intPart) + "."
	for i := 0; i < precision; i++ {
		fracPart *= 10
		digit := int64(fracPart)
		result += string(rune('0' + digit))
		fracPart -= float64(digit)
	}
	return result
}
