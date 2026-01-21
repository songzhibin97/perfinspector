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

	// 根据 profile 类型选择合适的值索引
	// 对于 CPU profile，优先使用 cpu/nanoseconds 类型的值
	valueIndex := 0
	useCumValue := false

	// 检查 SampleType 来选择最佳值索引
	if len(p.SampleType) > 1 {
		for i, st := range p.SampleType {
			if st.Type == "cpu" || st.Unit == "nanoseconds" {
				valueIndex = i
				useCumValue = true
				break
			}
		}
	} else if profileType == "cpu" && len(p.Sample) > 0 && len(p.Sample[0].Value) > 1 {
		valueIndex = 1 // 使用 cum 值
		useCumValue = true
	}

	// 计算总值（用于百分比计算）
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
		var chain CallChain
		if useCumValue {
			chain = a.extractor.ExtractCallChainWithCumValue(sample, totalValue)
		} else {
			chain = a.extractor.ExtractCallChain(sample, valueIndex, totalValue)
		}
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

	// 根据 profile 类型选择合适的值索引
	valueIndex := 0
	useCumValue := false

	// 检查第一个 profile 的 SampleType 来选择最佳值索引
	if len(profiles) > 0 && len(profiles[0].SampleType) > 1 {
		for i, st := range profiles[0].SampleType {
			if st.Type == "cpu" || st.Unit == "nanoseconds" {
				valueIndex = i
				useCumValue = true
				break
			}
		}
	} else if profileType == "cpu" && len(profiles) > 0 && len(profiles[0].Sample) > 0 && len(profiles[0].Sample[0].Value) > 1 {
		valueIndex = 1 // 使用 cum 值
		useCumValue = true
	}

	// 收集所有 profile 的热点路径
	allChains := make([]CallChain, 0)
	totalValueAcrossProfiles := int64(0)

	for _, p := range profiles {
		if p == nil || len(p.Sample) == 0 {
			continue
		}

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
			var chain CallChain
			if useCumValue {
				chain = a.extractor.ExtractCallChainWithCumValue(sample, profileTotalValue)
			} else {
				chain = a.extractor.ExtractCallChain(sample, valueIndex, profileTotalValue)
			}
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
		// 使用智能聚合策略：优先按业务代码聚合
		key := generateSmartCallChainKey(chain.Frames)

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

// generateSmartCallChainKey 生成智能调用链标识
// 策略：如果有业务代码，按业务代码聚合；否则按完整调用栈聚合
func generateSmartCallChainKey(frames []StackFrame) string {
	if len(frames) == 0 {
		return ""
	}

	// 查找业务代码帧
	businessFrames := make([]string, 0)
	for _, frame := range frames {
		if frame.Category == CategoryBusiness {
			businessFrames = append(businessFrames, frame.FunctionName)
		}
	}

	// 如果有业务代码，只使用业务代码部分作为 key
	// 这样可以将相同业务代码但底层调用不同的路径聚合在一起
	if len(businessFrames) > 0 {
		key := "business:"
		for i, fn := range businessFrames {
			if i > 0 {
				key += "|"
			}
			key += fn
		}
		return key
	}

	// 如果没有业务代码，使用完整调用栈
	// 但只使用前 5 帧，避免过度分散
	maxFrames := 5
	if len(frames) < maxFrames {
		maxFrames = len(frames)
	}

	key := "system:"
	for i := 0; i < maxFrames; i++ {
		if i > 0 {
			key += "|"
		}
		key += frames[i].FunctionName
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
