package locator

import (
	"sort"

	"github.com/google/pprof/profile"
)

// AnalyzeHotPathsImproved 改进的热点路径分析
// 使用 CPU 时间值而非采样次数，能更好地识别业务代码影响
func (a *PathAnalyzer) AnalyzeHotPathsImproved(p *profile.Profile, profileType string) []HotPath {
	if p == nil || len(p.Sample) == 0 {
		return nil
	}

	// 选择合适的值索引
	// 对于 CPU profile，优先使用 cpu/nanoseconds 类型的值
	valueIndex := selectBestValueIndex(p)

	// 计算总值
	totalValue := int64(0)
	for _, sample := range p.Sample {
		if len(sample.Value) > valueIndex {
			totalValue += sample.Value[valueIndex]
		}
	}

	if totalValue == 0 {
		return nil
	}

	// 提取所有调用链
	chains := make([]CallChain, 0, len(p.Sample))
	for _, sample := range p.Sample {
		chain := a.extractor.ExtractCallChain(sample, valueIndex, totalValue)
		if len(chain.Frames) > 0 {
			chains = append(chains, chain)
		}
	}

	// 聚合相同的调用链
	aggregated := a.AggregateCallChains(chains)

	// 按 TotalValue 降序排序
	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].TotalValue > aggregated[j].TotalValue
	})

	// 取 top N
	maxPaths := a.config.MaxHotPaths
	if len(aggregated) < maxPaths {
		maxPaths = len(aggregated)
	}
	topChains := aggregated[:maxPaths]

	// 转换为 HotPath
	hotPaths := make([]HotPath, 0, len(topChains))
	for _, chain := range topChains {
		// 限制调用栈深度
		if len(chain.Frames) > a.config.MaxCallStackDepth {
			chain.Frames = chain.Frames[:a.config.MaxCallStackDepth]
			chain.BoundaryPoints = FindBoundaryPoints(chain.Frames)
			chain.CategoryBreakdown = calculateCategoryBreakdown(chain.Frames)
		}

		businessFrames := FindBusinessFrames(chain.Frames)
		rootCauseIndex := -1
		if len(businessFrames) > 0 {
			rootCauseIndex = businessFrames[len(businessFrames)-1]
		}

		hotPaths = append(hotPaths, HotPath{
			Chain:          chain,
			BusinessFrames: businessFrames,
			RootCauseIndex: rootCauseIndex,
			ProfileType:    profileType,
		})
	}

	return hotPaths
}

// selectBestValueIndex 选择最佳的值索引
// 对于 CPU profile，优先选择 cpu/nanoseconds 类型
func selectBestValueIndex(p *profile.Profile) int {
	if len(p.SampleType) == 0 {
		return 0
	}

	// 优先查找 cpu 或 nanoseconds 类型
	for i, st := range p.SampleType {
		if st.Type == "cpu" || st.Unit == "nanoseconds" {
			return i
		}
	}

	// 如果没找到，使用第一个值
	return 0
}
