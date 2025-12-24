package rules

// Rule 表示一条分析规则
type Rule struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	ProfileTypes []string `yaml:"profile_types"`
	Condition    string   `yaml:"condition"`
	Actions      []Action `yaml:"actions"`
}

// CrossAnalysisRule 联合分析规则 - 跨多种 profile 类型的关联分析
type CrossAnalysisRule struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Conditions  map[string]string `yaml:"conditions"`  // 每种 profile 类型的条件
	Correlation string            `yaml:"correlation"` // 关联类型: same_direction, time_correlated
	Actions     []Action          `yaml:"actions"`
}

// Action 表示规则触发后的动作
type Action struct {
	Type             string            `yaml:"type"`
	Severity         string            `yaml:"severity"`
	Title            string            `yaml:"title"`
	EvidenceTemplate map[string]string `yaml:"evidence_template"`
	Suggestions      []string          `yaml:"suggestions"`
}

// Finding 表示规则匹配后的发现
type Finding struct {
	RuleID          string
	RuleName        string
	Severity        string
	Title           string
	Evidence        map[string]string
	Suggestions     []string
	IsCrossAnalysis bool // 是否为联合分析发现
}

// RulesConfig 规则配置文件结构
type RulesConfig struct {
	Rules              []Rule              `yaml:"rules"`
	CrossAnalysisRules []CrossAnalysisRule `yaml:"cross_analysis_rules"`
}
