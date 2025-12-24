package locator

// CodeCategory 代码分类
type CodeCategory string

const (
	CategoryRuntime    CodeCategory = "runtime"     // Go 运行时
	CategoryStdlib     CodeCategory = "stdlib"      // 标准库
	CategoryThirdParty CodeCategory = "third_party" // 第三方库
	CategoryBusiness   CodeCategory = "business"    // 业务代码
	CategoryUnknown    CodeCategory = "unknown"     // 未知
)

// String 返回分类的中文名称
func (c CodeCategory) String() string {
	switch c {
	case CategoryRuntime:
		return "运行时"
	case CategoryStdlib:
		return "标准库"
	case CategoryThirdParty:
		return "第三方"
	case CategoryBusiness:
		return "业务"
	default:
		return "未知"
	}
}

// Icon 返回分类的图标
func (c CodeCategory) Icon() string {
	switch c {
	case CategoryRuntime:
		return "⚙️"
	case CategoryStdlib:
		return "📚"
	case CategoryThirdParty:
		return "📦"
	case CategoryBusiness:
		return "💼"
	default:
		return "❓"
	}
}

// StackFrame 增强的栈帧信息
type StackFrame struct {
	FunctionName string       // 完整函数名 (包含包路径)
	ShortName    string       // 短函数名 (仅函数名)
	PackageName  string       // 包名
	FilePath     string       // 文件路径
	LineNumber   int64        // 行号
	Category     CodeCategory // 代码分类
	Flat         int64        // 自身消耗
	FlatPct      float64      // 自身消耗百分比
	Cum          int64        // 累计消耗（包含调用的函数）
	CumPct       float64      // 累计消耗百分比
}

// Location 返回 "文件:行号" 格式的位置字符串
func (f StackFrame) Location() string {
	if f.FilePath == "" || f.FilePath == "unknown" {
		return "unknown"
	}
	if f.LineNumber <= 0 {
		return f.FilePath
	}
	return f.FilePath + ":" + itoa(f.LineNumber)
}

// itoa 简单的 int64 转字符串
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// CallChain 完整调用链
type CallChain struct {
	Frames            []StackFrame         // 所有栈帧 (从入口到叶子)
	TotalValue        int64                // 总消耗值
	TotalPct          float64              // 总消耗百分比
	SampleCount       int                  // 样本数量
	CategoryBreakdown map[CodeCategory]int // 各类别帧数统计
	BoundaryPoints    []int                // 类别边界索引 (类别发生变化的位置)
}

// Summary 返回类别分布摘要字符串，如 "2 业务 → 1 第三方 → 2 标准库 → 3 运行时"
func (c CallChain) Summary() string {
	if len(c.Frames) == 0 {
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

	for _, frame := range c.Frames {
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

// HasBusinessCode 检查调用链是否包含业务代码
func (c CallChain) HasBusinessCode() bool {
	for _, frame := range c.Frames {
		if frame.Category == CategoryBusiness {
			return true
		}
	}
	return false
}

// HotPath 热点路径
type HotPath struct {
	Chain          CallChain // 调用链
	BusinessFrames []int     // 业务代码帧索引
	RootCauseIndex int       // 根因帧索引 (-1 表示无业务代码)
	ProfileType    string    // profile 类型 (cpu/heap/goroutine)
}

// GetRootCause 获取根因栈帧，如果没有业务代码则返回 nil
func (h HotPath) GetRootCause() *StackFrame {
	if h.RootCauseIndex < 0 || h.RootCauseIndex >= len(h.Chain.Frames) {
		return nil
	}
	return &h.Chain.Frames[h.RootCauseIndex]
}

// ExecutableCmd 可执行命令
type ExecutableCmd struct {
	Command     string // 命令内容
	Description string // 命令说明
	OutputHint  string // 输出解读提示
}

// Suggestion 建议
type Suggestion struct {
	Category string // "immediate" 或 "long_term"
	Content  string // 建议内容
}

// ProblemContext 问题上下文
type ProblemContext struct {
	Title       string          // 问题标题
	Severity    string          // 严重程度 (critical/high/medium/low)
	Explanation string          // 通俗解释
	Impact      string          // 影响评估
	HotPaths    []HotPath       // 热点路径列表
	Commands    []ExecutableCmd // 可执行命令
	Suggestions []Suggestion    // 建议列表
}

// LocatorConfig 定位器配置
type LocatorConfig struct {
	ModuleName         string   // 用户模块名 (从 go.mod 读取或手动指定)
	ThirdPartyPrefixes []string // 额外的第三方包前缀
	MaxCallStackDepth  int      // 最大调用栈深度 (默认 10)
	MaxHotPaths        int      // 最大热点路径数 (默认 5)
}

// DefaultConfig 返回默认配置
func DefaultConfig() LocatorConfig {
	return LocatorConfig{
		ModuleName:         "",
		ThirdPartyPrefixes: nil,
		MaxCallStackDepth:  10,
		MaxHotPaths:        5,
	}
}
