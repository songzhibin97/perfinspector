package locator

import (
	"strings"

	"github.com/google/pprof/profile"
)

// Extractor 调用栈提取器
type Extractor struct {
	classifier *Classifier
}

// NewExtractor 创建提取器
func NewExtractor(classifier *Classifier) *Extractor {
	return &Extractor{
		classifier: classifier,
	}
}

// ExtractPackageName 从函数全名提取包名
// 例如: "github.com/user/repo/pkg.(*Type).Method" -> "github.com/user/repo/pkg"
// 例如: "runtime.mallocgc" -> "runtime"
// 例如: "main.main" -> "main"
func ExtractPackageName(functionName string) string {
	if functionName == "" {
		return ""
	}

	// 处理方法接收者的情况: pkg.(*Type).Method 或 pkg.Type.Method
	// 找到最后一个 '/' 之后的第一个 '.'
	lastSlash := strings.LastIndex(functionName, "/")
	searchStart := 0
	if lastSlash >= 0 {
		searchStart = lastSlash + 1
	}

	// 在 searchStart 之后找第一个 '.'
	dotIndex := strings.Index(functionName[searchStart:], ".")
	if dotIndex < 0 {
		// 没有点，整个字符串就是包名（不太可能，但作为 fallback）
		return functionName
	}

	// 包名是从开头到第一个点（在最后一个斜杠之后）
	return functionName[:searchStart+dotIndex]
}

// ExtractShortName 从函数全名提取短名（仅函数/方法名）
// 例如: "github.com/user/repo/pkg.(*Type).Method" -> "(*Type).Method"
// 例如: "runtime.mallocgc" -> "mallocgc"
// 例如: "main.main" -> "main"
func ExtractShortName(functionName string) string {
	if functionName == "" {
		return ""
	}

	// 找到最后一个 '/' 之后的第一个 '.'
	lastSlash := strings.LastIndex(functionName, "/")
	searchStart := 0
	if lastSlash >= 0 {
		searchStart = lastSlash + 1
	}

	// 在 searchStart 之后找第一个 '.'
	dotIndex := strings.Index(functionName[searchStart:], ".")
	if dotIndex < 0 {
		// 没有点，返回整个字符串
		return functionName
	}

	// 短名是点之后的部分
	return functionName[searchStart+dotIndex+1:]
}

// ExtractStackFrame 从 pprof Location/Line 提取栈帧
// 如果 line 为 nil，则使用 location 的第一个 line（如果有）
func (e *Extractor) ExtractStackFrame(loc *profile.Location, line *profile.Line) StackFrame {
	frame := StackFrame{
		FunctionName: "unknown",
		ShortName:    "unknown",
		PackageName:  "",
		FilePath:     "unknown",
		LineNumber:   0,
		Category:     CategoryUnknown,
		Flat:         0,
		FlatPct:      0,
		Cum:          0,
		CumPct:       0,
	}

	// 如果 line 为 nil，尝试从 location 获取
	if line == nil && loc != nil && len(loc.Line) > 0 {
		line = &loc.Line[0]
	}

	if line == nil || line.Function == nil {
		return frame
	}

	fn := line.Function

	// 提取函数名
	if fn.Name != "" {
		frame.FunctionName = fn.Name
		frame.ShortName = ExtractShortName(fn.Name)
		frame.PackageName = ExtractPackageName(fn.Name)
	}

	// 提取文件路径
	if fn.Filename != "" {
		frame.FilePath = fn.Filename
	}

	// 提取行号
	if line.Line > 0 {
		frame.LineNumber = line.Line
	}

	// 分类
	if e.classifier != nil {
		frame.Category = e.classifier.Classify(frame.PackageName)
	}

	return frame
}

// ExtractCallChain 从 Sample 提取完整调用链
// valueIndex 指定使用哪个 value（通常 0 是 flat，1 是 cum，取决于 profile 类型）
// totalValue 是所有样本的总值，用于计算百分比
func (e *Extractor) ExtractCallChain(sample *profile.Sample, valueIndex int, totalValue int64) CallChain {
	chain := CallChain{
		Frames:            make([]StackFrame, 0),
		TotalValue:        0,
		TotalPct:          0,
		SampleCount:       1,
		CategoryBreakdown: make(map[CodeCategory]int),
		BoundaryPoints:    make([]int, 0),
	}

	if sample == nil {
		return chain
	}

	// 获取样本值
	if valueIndex >= 0 && valueIndex < len(sample.Value) {
		chain.TotalValue = sample.Value[valueIndex]
	}

	// 计算百分比
	if totalValue > 0 {
		chain.TotalPct = float64(chain.TotalValue) / float64(totalValue) * 100
	}

	// pprof 的 Location 列表是从叶子到根的顺序
	// 我们需要反转它，使其从入口点到叶子
	locations := sample.Location
	numLocs := len(locations)

	// 从后向前遍历（从入口点到叶子）
	var prevCategory CodeCategory
	for i := numLocs - 1; i >= 0; i-- {
		loc := locations[i]
		if loc == nil {
			continue
		}

		// 一个 Location 可能有多个 Line（内联函数）
		// 按照从外到内的顺序处理
		for j := len(loc.Line) - 1; j >= 0; j-- {
			line := &loc.Line[j]
			frame := e.ExtractStackFrame(loc, line)

			// 更新类别统计
			chain.CategoryBreakdown[frame.Category]++

			// 检测类别边界
			frameIndex := len(chain.Frames)
			if frameIndex > 0 && frame.Category != prevCategory {
				chain.BoundaryPoints = append(chain.BoundaryPoints, frameIndex)
			}
			prevCategory = frame.Category

			chain.Frames = append(chain.Frames, frame)
		}
	}

	return chain
}

// ExtractCallChainWithCumValue 从 Sample 提取完整调用链，使用累计值（cum）
// 对于 CPU profile，cum 值更能反映业务代码的影响
func (e *Extractor) ExtractCallChainWithCumValue(sample *profile.Sample, totalValue int64) CallChain {
	// CPU profile 通常有两个值：[flat, cum]
	// 我们使用 cum 值（索引 1）来识别业务代码的影响
	cumIndex := 1
	if len(sample.Value) <= cumIndex {
		// 如果没有 cum 值，回退到第一个值
		cumIndex = 0
	}

	chain := e.ExtractCallChain(sample, cumIndex, totalValue)
	return chain
}

// ExtractCallChainWithValues 从 Sample 提取完整调用链，并设置每帧的 flat/cum 值
// 这个方法用于需要显示每帧消耗值的场景
func (e *Extractor) ExtractCallChainWithValues(sample *profile.Sample, valueIndex int, totalValue int64, flatValues, cumValues map[uint64]int64) CallChain {
	chain := e.ExtractCallChain(sample, valueIndex, totalValue)

	// 如果提供了 flat/cum 值映射，更新每帧的值
	// 注意：由于 pprof 的结构，flat/cum 值通常在 profile 级别聚合
	// 这里预留接口供后续扩展
	_ = flatValues
	_ = cumValues

	return chain
}
