package locator

import (
	"sort"

	"github.com/google/pprof/profile"
)

// PathAnalyzer 热点路径分析器
type PathAnalyzer struct {
	extractor *Extractor
	config    LocatorConfig
}

// NewPathAnalyzer 创建分析器
func NewPathAnalyzer(extractor *Extractor, config LocatorConfig) *PathAnalyzer {
	// 设置默认值
	if config.MaxCallStackDepth <= 0 {
		config.MaxCallStackDepth = 10
	}
	if config.MaxHotPaths <= 0 {
		config.MaxHotPaths = 5
	}

	return &PathAnalyzer{
		extractor: extractor,
		config:    config,
	}
}

// AnalyzeHotPaths 分析热点路径，从 profile 提取 top N 热点路径
func (a *PathAnalyzer) AnalyzeHotPaths(p *profile.Profile, profileType string) []HotPath {
	if p == nil || len(p.Sample) == 0 {
		return nil
	}

	// 计算总值（用于百分比计算）
	valueIndex := 0 // 默认使用第一个 value
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
			// 重新计算边界点和类别统计
			chain.BoundaryPoints = FindBoundaryPoints(chain.Frames)
			chain.CategoryBreakdown = calculateCategoryBreakdown(chain.Frames)
		}

		businessFrames := FindBusinessFrames(chain.Frames)
		rootCauseIndex := -1
		if len(businessFrames) > 0 {
			// 根因是最深的业务代码帧（最接近热点的业务代码）
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

// AnalyzeMultipleProfiles 分析多个 profile 文件，综合所有热点函数
// 用于 CPU 热点分析，综合多个 profile 文件的结果
func (a *PathAnalyzer) AnalyzeMultipleProfiles(profiles []*profile.Profile, profileType string) []HotPath {
	if len(profiles) == 0 {
		return nil
	}

	// 如果只有一个 profile，直接分析
	if len(profiles) == 1 {
		return a.AnalyzeHotPaths(profiles[0], profileType)
	}

	// 收集所有 profile 的热点路径
	allChains := make([]CallChain, 0)
	totalValueAcrossProfiles := int64(0)

	for _, p := range profiles {
		if p == nil || len(p.Sample) == 0 {
			continue
		}

		valueIndex := 0
		profileTotalValue := int64(0)
		for _, sample := range p.Sample {
			if len(sample.Value) > valueIndex {
				profileTotalValue += sample.Value[valueIndex]
			}
		}

		if profileTotalValue == 0 {
			continue
		}

		totalValueAcrossProfiles += profileTotalValue

		// 提取该 profile 的所有调用链
		for _, sample := range p.Sample {
			chain := a.extractor.ExtractCallChain(sample, valueIndex, profileTotalValue)
			if len(chain.Frames) > 0 {
				allChains = append(allChains, chain)
			}
		}
	}

	if len(allChains) == 0 {
		return nil
	}

	// 聚合所有调用链
	aggregated := a.AggregateCallChains(allChains)

	// 重新计算百分比（基于所有 profile 的总值）
	for i := range aggregated {
		if totalValueAcrossProfiles > 0 {
			aggregated[i].TotalPct = float64(aggregated[i].TotalValue) / float64(totalValueAcrossProfiles) * 100
		}
	}

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

// AggregateCallChains 聚合相同调用路径的样本
// 相同调用路径的定义：所有帧的 FunctionName 完全相同
func (a *PathAnalyzer) AggregateCallChains(chains []CallChain) []CallChain {
	if len(chains) == 0 {
		return nil
	}

	// 使用调用路径签名作为 key 进行聚合
	aggregated := make(map[string]*CallChain)

	for i := range chains {
		chain := &chains[i]
		key := generateCallChainKey(chain.Frames)

		if existing, ok := aggregated[key]; ok {
			// 聚合：累加值和样本数
			existing.TotalValue += chain.TotalValue
			existing.TotalPct += chain.TotalPct
			existing.SampleCount += chain.SampleCount
		} else {
			// 创建新条目（复制以避免修改原始数据）
			newChain := CallChain{
				Frames:            make([]StackFrame, len(chain.Frames)),
				TotalValue:        chain.TotalValue,
				TotalPct:          chain.TotalPct,
				SampleCount:       chain.SampleCount,
				CategoryBreakdown: make(map[CodeCategory]int),
				BoundaryPoints:    make([]int, len(chain.BoundaryPoints)),
			}
			copy(newChain.Frames, chain.Frames)
			copy(newChain.BoundaryPoints, chain.BoundaryPoints)
			for k, v := range chain.CategoryBreakdown {
				newChain.CategoryBreakdown[k] = v
			}
			aggregated[key] = &newChain
		}
	}

	// 转换为切片
	result := make([]CallChain, 0, len(aggregated))
	for _, chain := range aggregated {
		result = append(result, *chain)
	}

	return result
}

// generateCallChainKey 生成调用链的唯一标识
func generateCallChainKey(frames []StackFrame) string {
	if len(frames) == 0 {
		return ""
	}

	// 使用所有帧的函数名拼接作为 key
	key := ""
	for i, frame := range frames {
		if i > 0 {
			key += "|"
		}
		key += frame.FunctionName
	}
	return key
}

// FindBoundaryPoints 找出类别边界索引
// 边界点是类别发生变化的位置（从索引 1 开始检查）
func FindBoundaryPoints(frames []StackFrame) []int {
	if len(frames) <= 1 {
		return nil
	}

	boundaries := make([]int, 0)
	for i := 1; i < len(frames); i++ {
		if frames[i].Category != frames[i-1].Category {
			boundaries = append(boundaries, i)
		}
	}
	return boundaries
}

// FindBusinessFrames 找出所有业务代码帧索引
// 返回的索引按升序排列（从入口到叶子）
func FindBusinessFrames(frames []StackFrame) []int {
	indices := make([]int, 0)
	for i, frame := range frames {
		if frame.Category == CategoryBusiness {
			indices = append(indices, i)
		}
	}
	return indices
}

// GenerateCategorySummary 生成类别分布摘要字符串
// 例如: "2 业务 → 1 第三方 → 2 标准库 → 3 运行时"
func GenerateCategorySummary(frames []StackFrame) string {
	if len(frames) == 0 {
		return "空调用链"
	}

	// 按顺序统计连续的类别段
	type segment struct {
		category CodeCategory
		count    int
	}
	var segments []segment

	var currentCategory CodeCategory
	var currentCount int

	for _, frame := range frames {
		if frame.Category != currentCategory {
			if currentCount > 0 {
				segments = append(segments, segment{currentCategory, currentCount})
			}
			currentCategory = frame.Category
			currentCount = 1
		} else {
			currentCount++
		}
	}
	if currentCount > 0 {
		segments = append(segments, segment{currentCategory, currentCount})
	}

	// 构建摘要字符串
	result := ""
	for i, seg := range segments {
		if i > 0 {
			result += " → "
		}
		result += itoa(int64(seg.count)) + " " + seg.category.String()
	}
	return result
}

// calculateCategoryBreakdown 计算类别分布统计
func calculateCategoryBreakdown(frames []StackFrame) map[CodeCategory]int {
	breakdown := make(map[CodeCategory]int)
	for _, frame := range frames {
		breakdown[frame.Category]++
	}
	return breakdown
}

// GetCategoryBreakdownSum 计算类别分布的总帧数
func GetCategoryBreakdownSum(breakdown map[CodeCategory]int) int {
	sum := 0
	for _, count := range breakdown {
		sum += count
	}
	return sum
}
