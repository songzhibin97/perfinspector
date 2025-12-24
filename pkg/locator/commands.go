package locator

import (
	"fmt"
	"strings"
)

// CommandGenerator 命令生成器
type CommandGenerator struct{}

// NewCommandGenerator 创建命令生成器
func NewCommandGenerator() *CommandGenerator {
	return &CommandGenerator{}
}

// GenerateCommands 根据 profile 类型和热点路径生成命令列表
func (g *CommandGenerator) GenerateCommands(
	profilePath string,
	profileType string,
	hotPaths []HotPath,
) []ExecutableCmd {
	commands := make([]ExecutableCmd, 0)

	// 确保 profilePath 不为空
	if profilePath == "" {
		profilePath = fmt.Sprintf("./%s.pprof", profileType)
	}

	// 基础命令 - 查看热点
	commands = append(commands, g.GenerateTopCommand(profilePath))

	// 如果有热点路径且有业务代码，生成聚焦命令
	if len(hotPaths) > 0 {
		topPath := hotPaths[0]
		if topPath.RootCauseIndex >= 0 && topPath.RootCauseIndex < len(topPath.Chain.Frames) {
			rootCause := topPath.Chain.Frames[topPath.RootCauseIndex]
			commands = append(commands, g.GenerateFocusCommand(profilePath, rootCause.ShortName))
			commands = append(commands, g.GenerateListCommand(profilePath, rootCause.ShortName))
		}
	}

	// Web 可视化命令
	commands = append(commands, g.GenerateWebCommand(profilePath))

	return commands
}

// GenerateFocusCommand 生成 -focus 命令，聚焦特定函数
func (g *CommandGenerator) GenerateFocusCommand(profilePath, functionName string) ExecutableCmd {
	// 清理函数名，移除可能的包路径前缀，只保留函数名部分
	shortName := extractShortFunctionName(functionName)

	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -focus=%s %s", shortName, profilePath),
		Description: fmt.Sprintf("聚焦到 %s 函数，只显示包含该函数的调用路径", shortName),
		OutputHint:  "输出将只显示经过指定函数的调用路径，帮助你理解该函数的调用上下文",
	}
}

// GenerateTopCommand 生成 -top 命令，查看热点函数列表
func (g *CommandGenerator) GenerateTopCommand(profilePath string) ExecutableCmd {
	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -top %s", profilePath),
		Description: "查看消耗最多资源的函数列表",
		OutputHint:  "flat 列显示函数自身消耗，cum 列显示函数及其调用的所有函数的总消耗",
	}
}

// GenerateListCommand 生成 -list 命令，查看源码级别分析
func (g *CommandGenerator) GenerateListCommand(profilePath, functionName string) ExecutableCmd {
	shortName := extractShortFunctionName(functionName)

	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -list=%s %s", shortName, profilePath),
		Description: fmt.Sprintf("查看 %s 函数的源码级别分析", shortName),
		OutputHint:  "显示函数源码及每行的资源消耗，帮助定位具体的问题代码行",
	}
}

// GenerateWebCommand 生成 -http 命令，启动 Web 可视化界面
func (g *CommandGenerator) GenerateWebCommand(profilePath string) ExecutableCmd {
	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -http=:8080 %s", profilePath),
		Description: "在浏览器中打开交互式可视化界面",
		OutputHint:  "提供火焰图、调用图等多种可视化方式，支持交互式探索",
	}
}

// GenerateDiffCommand 生成差异对比命令
// basePath: 基准 profile 文件路径
// targetPath: 目标 profile 文件路径
func (g *CommandGenerator) GenerateDiffCommand(basePath, targetPath string) ExecutableCmd {
	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -base=%s %s", basePath, targetPath),
		Description: "对比两个 profile 文件的差异，查看资源消耗的变化",
		OutputHint:  "正值表示目标 profile 比基准 profile 消耗更多，负值表示消耗减少",
	}
}

// GenerateCommandsWithContext 根据完整上下文生成命令
// profilePaths: profile 文件路径列表
// profileType: profile 类型 (cpu/heap/goroutine)
// hotPaths: 热点路径列表
// 返回针对性的 pprof 命令列表
func (g *CommandGenerator) GenerateCommandsWithContext(
	profilePaths []string,
	profileType string,
	hotPaths []HotPath,
) []ExecutableCmd {
	commands := make([]ExecutableCmd, 0)

	// 确保至少有一个 profile 路径
	if len(profilePaths) == 0 {
		profilePaths = []string{fmt.Sprintf("./%s.pprof", profileType)}
	}

	// 使用第一个 profile 作为主要分析目标
	primaryPath := profilePaths[0]

	// 基础命令 - 查看热点
	commands = append(commands, g.GenerateTopCommand(primaryPath))

	// 根据 profile 类型添加特定命令
	switch profileType {
	case "heap":
		commands = append(commands, g.GenerateAllocSpaceCommand(primaryPath))
		commands = append(commands, g.GenerateInuseSpaceCommand(primaryPath))
	case "goroutine":
		// goroutine profile 特定命令 - 聚焦阻塞函数
		if len(hotPaths) > 0 {
			for _, hp := range hotPaths {
				if hp.RootCauseIndex >= 0 && hp.RootCauseIndex < len(hp.Chain.Frames) {
					rootCause := hp.Chain.Frames[hp.RootCauseIndex]
					// 检查是否是阻塞相关函数
					if isBlockingFunction(rootCause.FunctionName) {
						commands = append(commands, g.GenerateFocusCommand(primaryPath, rootCause.ShortName))
						break
					}
				}
			}
		}
	}

	// 如果有热点路径且有业务代码，生成聚焦命令
	if len(hotPaths) > 0 {
		topPath := hotPaths[0]
		if topPath.RootCauseIndex >= 0 && topPath.RootCauseIndex < len(topPath.Chain.Frames) {
			rootCause := topPath.Chain.Frames[topPath.RootCauseIndex]
			// 避免重复添加 focus 命令
			if !containsFocusCommand(commands, rootCause.ShortName) {
				commands = append(commands, g.GenerateFocusCommand(primaryPath, rootCause.ShortName))
			}
			commands = append(commands, g.GenerateListCommand(primaryPath, rootCause.ShortName))
		}
	}

	// 如果有多个 profile 文件，生成差异对比命令
	if len(profilePaths) >= 2 {
		// 使用第一个作为基准，最后一个作为目标
		basePath := profilePaths[0]
		targetPath := profilePaths[len(profilePaths)-1]
		commands = append(commands, g.GenerateDiffCommand(basePath, targetPath))
	}

	// Web 可视化命令
	commands = append(commands, g.GenerateWebCommand(primaryPath))

	return commands
}

// isBlockingFunction 检查是否是阻塞相关函数
func isBlockingFunction(functionName string) bool {
	blockingPatterns := []string{
		"chansend", "chanrecv", "select", "semacquire",
		"Lock", "RLock", "Wait", "Sleep",
	}
	for _, pattern := range blockingPatterns {
		if strings.Contains(functionName, pattern) {
			return true
		}
	}
	return false
}

// containsFocusCommand 检查命令列表中是否已包含指定函数的 focus 命令
func containsFocusCommand(commands []ExecutableCmd, functionName string) bool {
	focusPattern := fmt.Sprintf("-focus=%s", functionName)
	for _, cmd := range commands {
		if strings.Contains(cmd.Command, focusPattern) {
			return true
		}
	}
	return false
}

// GenerateAllocSpaceCommand 生成内存分配分析命令（仅用于 heap profile）
func (g *CommandGenerator) GenerateAllocSpaceCommand(profilePath string) ExecutableCmd {
	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -alloc_space %s", profilePath),
		Description: "查看累计分配的内存，找出分配最多的函数",
		OutputHint:  "显示程序运行期间累计分配的内存量，帮助发现内存分配热点",
	}
}

// GenerateInuseSpaceCommand 生成内存使用分析命令（仅用于 heap profile）
func (g *CommandGenerator) GenerateInuseSpaceCommand(profilePath string) ExecutableCmd {
	return ExecutableCmd{
		Command:     fmt.Sprintf("go tool pprof -inuse_space %s", profilePath),
		Description: "查看当前正在使用的内存",
		OutputHint:  "显示当前仍在使用的内存量，帮助发现内存泄漏",
	}
}

// GenerateCommandsForProfileType 根据 profile 类型生成特定的命令集
func (g *CommandGenerator) GenerateCommandsForProfileType(
	profilePath string,
	profileType string,
	hotPaths []HotPath,
) []ExecutableCmd {
	commands := g.GenerateCommands(profilePath, profileType, hotPaths)

	// 根据 profile 类型添加特定命令
	switch profileType {
	case "heap":
		// 在 top 命令后插入内存特定命令
		heapCommands := []ExecutableCmd{
			g.GenerateAllocSpaceCommand(profilePath),
			g.GenerateInuseSpaceCommand(profilePath),
		}
		// 插入到第二个位置
		if len(commands) > 1 {
			newCommands := make([]ExecutableCmd, 0, len(commands)+len(heapCommands))
			newCommands = append(newCommands, commands[0])
			newCommands = append(newCommands, heapCommands...)
			newCommands = append(newCommands, commands[1:]...)
			commands = newCommands
		} else {
			commands = append(commands, heapCommands...)
		}
	}

	return commands
}

// extractShortFunctionName 从完整函数名中提取短函数名
func extractShortFunctionName(functionName string) string {
	if functionName == "" {
		return functionName
	}

	// 尝试提取最后一个 . 后面的部分（方法名）
	// 例如: "github.com/user/pkg.(*Type).Method" -> "Method"
	// 或者: "main.handleRequest" -> "handleRequest"
	// 或者: "github.com/user/pkg.HandleRequest" -> "HandleRequest"
	// 特殊处理匿名函数:
	// "main.init.0.func1.1" -> "init.0.func1.1" (保留匿名函数的完整标识)
	// "main.createWorker.func1" -> "createWorker.func1"

	// 首先检查是否是匿名函数（包含 .func 或以数字结尾）
	if isAnonymousFunction(functionName) {
		return extractAnonymousFunctionName(functionName)
	}

	lastDot := -1
	parenDepth := 0
	for i := len(functionName) - 1; i >= 0; i-- {
		c := functionName[i]
		if c == ')' {
			parenDepth++
		} else if c == '(' {
			parenDepth--
		} else if c == '.' && parenDepth == 0 {
			lastDot = i
			break
		}
	}

	if lastDot >= 0 && lastDot < len(functionName)-1 {
		return functionName[lastDot+1:]
	}

	return functionName
}

// isAnonymousFunction 检查是否是匿名函数
func isAnonymousFunction(functionName string) bool {
	// 匿名函数的特征：
	// 1. 包含 ".func" 模式（如 main.init.func1, createWorker.func1）
	// 2. 以数字结尾（如 init.0.func1.1）
	if strings.Contains(functionName, ".func") {
		return true
	}
	// 检查是否以数字结尾
	if len(functionName) > 0 {
		lastChar := functionName[len(functionName)-1]
		if lastChar >= '0' && lastChar <= '9' {
			// 进一步检查是否是 .数字 模式
			for i := len(functionName) - 1; i >= 0; i-- {
				c := functionName[i]
				if c == '.' {
					// 找到了 .数字 模式，可能是匿名函数
					return true
				}
				if c < '0' || c > '9' {
					break
				}
			}
		}
	}
	return false
}

// extractAnonymousFunctionName 提取匿名函数的有意义名称
func extractAnonymousFunctionName(functionName string) string {
	// 对于匿名函数，我们需要保留足够的上下文
	// 例如: "main.init.0.func1.1" -> "init.0.func1.1"
	// 例如: "main.createWorker.func1" -> "createWorker.func1"
	// 例如: "github.com/user/pkg.(*Server).handleRequest.func1" -> "handleRequest.func1"

	// 找到 .func 的位置
	funcIndex := strings.Index(functionName, ".func")
	if funcIndex == -1 {
		// 没有 .func，可能是 init.0 这样的模式
		// 尝试找到包名后的部分
		return extractAfterPackage(functionName)
	}

	// 从 .func 往前找，找到父函数名
	// 跳过可能的 (*Type) 部分
	startIndex := 0
	parenDepth := 0
	for i := funcIndex - 1; i >= 0; i-- {
		c := functionName[i]
		if c == ')' {
			parenDepth++
		} else if c == '(' {
			parenDepth--
		} else if c == '.' && parenDepth == 0 {
			// 检查这个点之前是否是包路径
			prefix := functionName[:i]
			if strings.Contains(prefix, "/") || prefix == "main" || prefix == "runtime" {
				startIndex = i + 1
				break
			}
		}
	}

	result := functionName[startIndex:]
	if result == "" {
		return functionName
	}
	return result
}

// extractAfterPackage 提取包名之后的部分
func extractAfterPackage(functionName string) string {
	// 找到最后一个 / 之后的第一个 .
	lastSlash := strings.LastIndex(functionName, "/")
	if lastSlash >= 0 {
		remainder := functionName[lastSlash+1:]
		dotIndex := strings.Index(remainder, ".")
		if dotIndex >= 0 {
			return remainder[dotIndex+1:]
		}
		return remainder
	}

	// 没有 /，可能是 main.xxx 或 runtime.xxx
	dotIndex := strings.Index(functionName, ".")
	if dotIndex >= 0 {
		return functionName[dotIndex+1:]
	}

	return functionName
}
