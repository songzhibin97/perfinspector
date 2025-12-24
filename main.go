package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/songzhibin97/perfinspector/pkg/reporter"
	"github.com/songzhibin97/perfinspector/pkg/rules"
)

// Config 命令行配置
type Config struct {
	InputPath  string // 输入路径（目录或文件）
	Format     string // 输出格式: text, html
	OutputPath string // 输出文件路径
	RulesPath  string // 规则文件路径

	// Problem Locator 配置
	ModuleName         string   // 用户模块名
	ThirdPartyPrefixes []string // 额外的第三方包前缀
	StackDepth         int      // 最大调用栈深度
	HotPaths           int      // 最大热点路径数
}

// DefaultRulesPath 默认规则文件路径
const DefaultRulesPath = "assets/default_rules.yaml"

func main() {
	config, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	paths, err := getProfilePaths(config.InputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "No valid profile files found")
		os.Exit(1)
	}

	// 分组分析
	groups, err := analyzer.GroupProfiles(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Analysis failed: %v\n", err)
		os.Exit(1)
	}

	// 计算趋势
	trends := make(map[string]*analyzer.GroupTrends)
	for _, group := range groups {
		if t := analyzer.CalculateTrends(group); t != nil {
			trends[group.Type] = t
		}
	}

	// 加载规则引擎
	var findings []rules.Finding
	engine, err := rules.NewEngine(config.RulesPath)
	if err != nil {
		// 规则加载失败只是警告，不影响主流程
		fmt.Fprintf(os.Stderr, "⚠️ 规则加载失败: %v\n", err)
	} else if engine != nil {
		findings = engine.Evaluate(groups, trends)
	}

	// 初始化 Problem Locator
	locatorConfig := createLocatorConfig(config)
	contexts := generateProblemContexts(findings, groups, locatorConfig)

	// 生成报告
	switch config.Format {
	case "html":
		outputPath := config.OutputPath
		if outputPath == "" {
			outputPath = "report.html"
		}
		if err := reporter.GenerateHTMLReportWithContext(groups, trends, findings, contexts, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "HTML report generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ HTML 报告已生成: %s\n", outputPath)
	default:
		reporter.GenerateTextReportWithContext(groups, trends, findings, contexts)
	}
}

// parseArgs 解析命令行参数
func parseArgs() (*Config, error) {
	config := &Config{}

	// 基础配置
	flag.StringVar(&config.Format, "format", "text", "输出格式: text, html")
	flag.StringVar(&config.OutputPath, "output", "", "输出文件路径")
	flag.StringVar(&config.RulesPath, "rules", DefaultRulesPath, "规则文件路径")

	// Problem Locator 配置
	flag.StringVar(&config.ModuleName, "module", "", "用户模块名 (默认从 go.mod 自动检测)")
	var thirdPartyPrefixes string
	flag.StringVar(&thirdPartyPrefixes, "third-party-prefixes", "", "额外的第三方包前缀，逗号分隔")
	flag.IntVar(&config.StackDepth, "stack-depth", 10, "最大调用栈深度 (默认 10)")
	flag.IntVar(&config.HotPaths, "hot-paths", 5, "最大热点路径数 (默认 5)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "PerfInspector v0.1 - 智能时间序列 pprof 分析工具\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <profile_dir_or_file>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s ./profiles/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -format html -output report.html ./profiles/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -rules custom_rules.yaml ./profiles/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -module github.com/myorg/myapp -stack-depth 15 ./profiles/\n", os.Args[0])
	}

	flag.Parse()

	// 验证 format 参数
	if config.Format != "text" && config.Format != "html" {
		return nil, fmt.Errorf("invalid format '%s', must be 'text' or 'html'", config.Format)
	}

	// 解析第三方包前缀
	if thirdPartyPrefixes != "" {
		config.ThirdPartyPrefixes = strings.Split(thirdPartyPrefixes, ",")
		// 去除空白
		for i := range config.ThirdPartyPrefixes {
			config.ThirdPartyPrefixes[i] = strings.TrimSpace(config.ThirdPartyPrefixes[i])
		}
	}

	// 验证配置限制
	if config.StackDepth < 1 {
		config.StackDepth = 1
	}
	if config.StackDepth > 100 {
		config.StackDepth = 100
	}
	if config.HotPaths < 1 {
		config.HotPaths = 1
	}
	if config.HotPaths > 50 {
		config.HotPaths = 50
	}

	// 获取输入路径
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return nil, fmt.Errorf("missing input path")
	}
	config.InputPath = args[0]

	return config, nil
}

func getProfilePaths(path string) ([]string, error) {
	var paths []string
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && isProfileFile(p) {
				paths = append(paths, p)
			}
			return nil
		})
	} else if isProfileFile(path) {
		paths = []string{path}
	} else {
		return nil, fmt.Errorf("path is not a directory or valid profile file")
	}

	return paths, err
}

func isProfileFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".pprof" || ext == ".profile"
}

// createLocatorConfig 创建 Problem Locator 配置
func createLocatorConfig(config *Config) locator.LocatorConfig {
	locatorConfig := locator.DefaultConfig()

	// 设置模块名
	if config.ModuleName != "" {
		locatorConfig.ModuleName = config.ModuleName
	} else {
		// 尝试从 go.mod 自动检测
		if moduleName, err := locator.DetectModuleName("."); err == nil {
			locatorConfig.ModuleName = moduleName
		}
	}

	// 设置第三方包前缀
	if len(config.ThirdPartyPrefixes) > 0 {
		locatorConfig.ThirdPartyPrefixes = config.ThirdPartyPrefixes
	}

	// 设置调用栈深度和热点路径数
	locatorConfig.MaxCallStackDepth = config.StackDepth
	locatorConfig.MaxHotPaths = config.HotPaths

	return locatorConfig
}

// generateProblemContexts 为每个 Finding 生成 ProblemContext
func generateProblemContexts(findings []rules.Finding, groups []analyzer.ProfileGroup, config locator.LocatorConfig) map[string]*locator.ProblemContext {
	if len(findings) == 0 {
		return nil
	}

	// 创建 locator 组件
	classifier := locator.NewClassifier(config)
	extractor := locator.NewExtractor(classifier)
	pathAnalyzer := locator.NewPathAnalyzer(extractor, config)
	contextGenerator := locator.NewContextGenerator(pathAnalyzer)

	// 收集所有 profiles，按类型组织（用于向后兼容，保留最新的单个 profile）
	profiles := make(map[string]*profile.Profile)
	// 收集所有 profiles，按类型组织（用于综合分析）
	allProfiles := make(map[string][]*profile.Profile)
	// 收集所有 profile 文件路径，按类型组织
	profilePaths := make(map[string][]string)

	for _, group := range groups {
		if len(group.Files) > 0 {
			// 使用最新的 profile（最后一个）- 向后兼容
			profiles[group.Type] = group.Files[len(group.Files)-1].Profile

			// 收集该类型的所有 profiles（用于综合分析）
			for _, file := range group.Files {
				if file.Profile != nil {
					allProfiles[group.Type] = append(allProfiles[group.Type], file.Profile)
				}
				profilePaths[group.Type] = append(profilePaths[group.Type], file.Path)
			}
		}
	}

	// 为每个 Finding 生成 ProblemContext
	contexts := make(map[string]*locator.ProblemContext)
	for _, finding := range findings {
		// 确定该 finding 对应的 profile 类型
		profileType := determineProfileTypeFromFinding(finding)
		// 获取对应类型的 profile 路径
		paths := profilePaths[profileType]
		// 使用新的综合分析方法
		ctx := contextGenerator.GenerateContextWithAllProfiles(finding, profiles, allProfiles, paths)
		if ctx != nil {
			contexts[finding.RuleID] = ctx
		}
	}

	return contexts
}

// determineProfileTypeFromFinding 从 Finding 确定 profile 类型
func determineProfileTypeFromFinding(finding rules.Finding) string {
	title := strings.ToLower(finding.Title)
	ruleID := strings.ToLower(finding.RuleID)

	if strings.Contains(title, "cpu") || strings.Contains(ruleID, "cpu") {
		return "cpu"
	}
	if strings.Contains(title, "内存") || strings.Contains(title, "memory") ||
		strings.Contains(title, "heap") || strings.Contains(ruleID, "heap") ||
		strings.Contains(ruleID, "memory") {
		return "heap"
	}
	if strings.Contains(title, "goroutine") || strings.Contains(ruleID, "goroutine") ||
		strings.Contains(title, "协程") {
		return "goroutine"
	}

	return "cpu"
}
