package reporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/songzhibin97/perfinspector/pkg/analyzer"
	"github.com/songzhibin97/perfinspector/pkg/locator"
	"github.com/songzhibin97/perfinspector/pkg/rules"
)

// HTMLReportData HTML 报告数据
type HTMLReportData struct {
	Title           string
	Version         string
	Generated       string
	Groups          []HTMLGroupData
	Findings        []rules.Finding
	ProblemContexts map[string]*HTMLProblemContext // 问题上下文映射 (RuleID -> HTMLProblemContext)
}

// HTMLGroupData HTML 报告中的分组数据
type HTMLGroupData struct {
	Type      string
	Files     []HTMLFileData
	TimeRange string
	Duration  string
	HasTrends bool
	Trends    *analyzer.GroupTrends
	ChartData []HTMLChartPoint       // 图表数据点
	ChartType string                 // "heap" 或 "goroutine"
	ChartUnit string                 // 单位显示
	ChartMax  float64                // Y轴最大值
	ChartMin  float64                // Y轴最小值
	Insights  []analyzer.HeapInsight // 智能洞察
}

// HTMLChartPoint 图表数据点
type HTMLChartPoint struct {
	Index      int     // 序号
	Value      float64 // 原始值
	Normalized float64 // 归一化值 (0-100)
	Label      string  // 显示标签
	Time       string  // 时间标签
}

// HTMLFileData HTML 报告中的文件数据
type HTMLFileData struct {
	Name        string
	Time        string
	Size        string
	Metrics     *analyzer.ProfileMetrics
	ProfileType string
}

// HTMLHotPath HTML 报告中的热点路径数据
type HTMLHotPath struct {
	Index          int
	TotalPct       float64
	Summary        string
	Frames         []HTMLStackFrame
	HasBusiness    bool
	RootCauseIndex int
}

// HTMLStackFrame HTML 报告中的栈帧数据
type HTMLStackFrame struct {
	Index        int
	Category     string
	CategoryIcon string
	ShortName    string
	Location     string
	FileLink     template.URL // Use template.URL to allow file:// protocol
	IsHighlight  bool
	HighlightTag string
	IsNewSection bool
}

// HTMLExecutableCmd HTML 报告中的可执行命令
type HTMLExecutableCmd struct {
	Index       int
	Command     string
	Description string
	OutputHint  string
}

// HTMLSuggestion HTML 报告中的建议
type HTMLSuggestion struct {
	Category string
	Content  string
}

// HTMLProblemContext HTML 报告中的问题上下文
type HTMLProblemContext struct {
	Title                string
	Severity             string
	Explanation          string
	Impact               string
	HotPaths             []HTMLHotPath
	Commands             []HTMLExecutableCmd
	ImmediateSuggestions []HTMLSuggestion
	LongTermSuggestions  []HTMLSuggestion
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header {
            background: white;
            border-radius: 16px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
            text-align: center;
        }
        .header h1 { color: #333; font-size: 2em; margin-bottom: 10px; }
        .header .version { color: #667eea; font-weight: 600; }
        .header .generated { color: #666; font-size: 0.9em; margin-top: 10px; }
        .group {
            background: white;
            border-radius: 16px;
            padding: 25px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
        }
        .group-header {
            display: flex;
            align-items: center;
            margin-bottom: 20px;
            padding-bottom: 15px;
            border-bottom: 2px solid #f0f0f0;
        }
        .group-icon { font-size: 2em; margin-right: 15px; }
        .group-title { font-size: 1.4em; color: #333; }
        .group-count {
            background: #667eea;
            color: white;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 0.85em;
            margin-left: 15px;
        }
        .file-card {
            background: #f8f9fa;
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 15px;
            border-left: 4px solid #667eea;
        }
        .file-header {
            display: flex;
            align-items: center;
            margin-bottom: 15px;
        }
        .file-number {
            background: #667eea;
            color: white;
            width: 32px;
            height: 32px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 600;
            margin-right: 15px;
        }
        .file-name { font-weight: 600; color: #333; font-size: 1.1em; }
        .file-meta {
            display: flex;
            gap: 20px;
            font-size: 0.9em;
            color: #666;
            margin-bottom: 15px;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 15px;
        }
        .metric-card {
            background: white;
            border-radius: 8px;
            padding: 15px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.05);
        }
        .metric-label { font-size: 0.8em; color: #888; margin-bottom: 5px; }
        .metric-value { font-size: 1.3em; font-weight: 600; color: #333; }
        .metric-value.highlight { color: #667eea; }
        .top-functions {
            background: white;
            border-radius: 8px;
            padding: 15px;
        }
        .top-functions h4 {
            font-size: 0.9em;
            color: #666;
            margin-bottom: 10px;
            display: flex;
            align-items: center;
        }
        .top-functions h4::before { content: "🔥"; margin-right: 8px; }
        .func-item {
            display: flex;
            align-items: center;
            padding: 8px 0;
            border-bottom: 1px solid #f0f0f0;
        }
        .func-item:last-child { border-bottom: none; }
        .func-rank {
            width: 24px;
            height: 24px;
            background: #e9ecef;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 0.75em;
            font-weight: 600;
            margin-right: 10px;
        }
        .func-rank.top1 { background: #ffd700; color: #333; }
        .func-rank.top2 { background: #c0c0c0; color: #333; }
        .func-rank.top3 { background: #cd7f32; color: white; }
        .func-name {
            flex: 1;
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.85em;
            color: #333;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .func-pct {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 0.75em;
            font-weight: 600;
        }
        
        /* Insights Section */
        .insights-section {
            margin: 20px 0;
        }
        .insights-section h3 {
            font-size: 1.2em;
            color: #333;
            margin-bottom: 15px;
        }
        .insight-card {
            background: white;
            border-radius: 8px;
            padding: 15px;
            margin-bottom: 15px;
            border-left: 4px solid #667eea;
        }
        .insight-card.critical {
            border-left-color: #e74c3c;
            background: #fff5f5;
        }
        .insight-card.warning {
            border-left-color: #f39c12;
            background: #fffbf0;
        }
        .insight-card.info {
            border-left-color: #3498db;
            background: #f0f8ff;
        }
        .insight-header {
            display: flex;
            align-items: center;
            margin-bottom: 10px;
        }
        .insight-icon {
            font-size: 1.2em;
            margin-right: 10px;
        }
        .insight-title {
            font-weight: 600;
            font-size: 1em;
            color: #333;
        }
        .insight-description {
            color: #666;
            margin-bottom: 10px;
            line-height: 1.5;
        }
        .insight-suggestions {
            background: rgba(255, 255, 255, 0.7);
            padding: 10px;
            border-radius: 4px;
        }
        .insight-suggestions strong {
            color: #333;
            display: block;
            margin-bottom: 5px;
        }
        .insight-suggestions ul {
            margin: 0;
            padding-left: 20px;
        }
        .insight-suggestions li {
            color: #555;
            margin: 5px 0;
            line-height: 1.4;
        }
        
        .stats {
            display: flex;
            gap: 15px;
            margin-top: 20px;
            padding-top: 15px;
            border-top: 2px solid #f0f0f0;
            flex-wrap: wrap;
        }
        .stat-item {
            display: flex;
            align-items: center;
            padding: 10px 15px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            border-radius: 8px;
            color: white;
        }
        .stat-icon { font-size: 1.2em; margin-right: 10px; }
        .stat-label { font-size: 0.85em; opacity: 0.9; }
        .stat-value { font-weight: 600; margin-left: 8px; }
        .trends {
            margin-top: 20px;
            padding: 20px;
            background: linear-gradient(135deg, #fff3cd 0%, #ffeeba 100%);
            border-radius: 12px;
            border-left: 4px solid #ffc107;
        }
        .trends h4 { color: #856404; margin-bottom: 15px; font-size: 1.1em; }
        .trend-item {
            display: flex;
            align-items: center;
            padding: 10px;
            background: white;
            border-radius: 8px;
            margin-bottom: 10px;
        }
        .trend-icon { font-size: 1.5em; margin-right: 15px; }
        .trend-details { flex: 1; }
        .trend-label { font-weight: 600; color: #333; }
        .trend-stats { font-size: 0.85em; color: #666; margin-top: 5px; }
        .findings {
            background: white;
            border-radius: 16px;
            padding: 25px;
            margin-bottom: 20px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
        }
        .findings-header {
            display: flex;
            align-items: center;
            margin-bottom: 20px;
            padding-bottom: 15px;
            border-bottom: 2px solid #f0f0f0;
        }
        .finding-item {
            padding: 20px;
            margin-bottom: 15px;
            border-radius: 12px;
            border-left: 4px solid;
        }
        .finding-critical { background: linear-gradient(135deg, #f5c6cb 0%, #f1b0b7 100%); border-color: #721c24; }
        .finding-high { background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-color: #dc3545; }
        .finding-medium { background: linear-gradient(135deg, #fff3cd 0%, #ffeeba 100%); border-color: #ffc107; }
        .finding-low { background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-color: #28a745; }
        .finding-title { font-weight: 600; font-size: 1.1em; margin-bottom: 10px; }
        .finding-meta { font-size: 0.85em; color: #666; margin-bottom: 15px; }
        .suggestions { margin-top: 15px; }
        .suggestions h5 { font-size: 0.9em; color: #333; margin-bottom: 10px; }
        .suggestions ul { margin-left: 20px; font-size: 0.9em; color: #555; }
        .suggestions li { margin-bottom: 5px; }

        /* Problem Locator 样式 - 代码分类颜色 */
        .frame-runtime { 
            background: linear-gradient(135deg, #6c757d 0%, #5a6268 100%);
            color: white;
        }
        .frame-stdlib { 
            background: linear-gradient(135deg, #17a2b8 0%, #138496 100%);
            color: white;
        }
        .frame-third-party { 
            background: linear-gradient(135deg, #6f42c1 0%, #5a32a3 100%);
            color: white;
        }
        .frame-business { 
            background: linear-gradient(135deg, #28a745 0%, #1e7e34 100%);
            color: white;
        }
        .frame-unknown { 
            background: linear-gradient(135deg, #adb5bd 0%, #868e96 100%);
            color: white;
        }

        /* 问题上下文样式 */
        .problem-context {
            background: #f8f9fa;
            border-radius: 12px;
            padding: 20px;
            margin-top: 15px;
        }
        .problem-explanation {
            background: white;
            border-radius: 8px;
            padding: 15px;
            margin-bottom: 15px;
            border-left: 4px solid #667eea;
        }
        .problem-explanation h5 { color: #667eea; margin-bottom: 10px; }
        .problem-explanation p { color: #555; line-height: 1.6; }
        .problem-impact {
            background: white;
            border-radius: 8px;
            padding: 15px;
            margin-bottom: 15px;
            border-left: 4px solid #ffc107;
        }
        .problem-impact h5 { color: #856404; margin-bottom: 10px; }
        .problem-impact p { color: #555; }

        /* 热点路径样式 */
        .hot-paths { margin-top: 20px; }
        .hot-paths h5 { color: #dc3545; margin-bottom: 15px; font-size: 1em; }
        .hot-path-item {
            background: white;
            border-radius: 8px;
            margin-bottom: 15px;
            overflow: hidden;
        }
        .hot-path-header {
            padding: 15px;
            background: linear-gradient(135deg, #ff6b6b 0%, #ee5a5a 100%);
            color: white;
            cursor: pointer;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .hot-path-header:hover { opacity: 0.9; }
        .hot-path-title { font-weight: 600; }
        .hot-path-pct { 
            background: rgba(255,255,255,0.2);
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
        }
        .hot-path-summary {
            padding: 10px 15px;
            background: #f8f9fa;
            font-size: 0.85em;
            color: #666;
            border-bottom: 1px solid #e9ecef;
        }
        .call-chain {
            padding: 15px;
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
            font-size: 0.85em;
        }
        .call-chain-frame {
            display: flex;
            align-items: flex-start;
            padding: 8px 0;
            border-bottom: 1px solid #f0f0f0;
        }
        .call-chain-frame:last-child { border-bottom: none; }
        .call-chain-frame.highlight {
            background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%);
            margin: 0 -15px;
            padding: 8px 15px;
            border-radius: 4px;
        }
        .frame-category {
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.75em;
            margin-right: 10px;
            min-width: 60px;
            text-align: center;
        }
        .frame-info { flex: 1; }
        .frame-name { color: #333; }
        .frame-location { 
            color: #667eea; 
            font-size: 0.9em;
            margin-top: 4px;
        }
        .frame-location a { 
            color: #667eea; 
            text-decoration: none;
        }
        .frame-location a:hover { text-decoration: underline; }
        .frame-tag {
            background: #28a745;
            color: white;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.75em;
            margin-left: 10px;
        }
        .frame-tag.root-cause { background: #dc3545; }
        .section-divider {
            text-align: center;
            padding: 8px 0;
            color: #adb5bd;
            font-size: 0.8em;
        }
        .no-business-warning {
            background: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 8px;
            padding: 12px;
            margin-top: 10px;
            color: #856404;
            font-size: 0.9em;
        }
        .no-business-warning ul {
            list-style-type: disc;
        }
        .no-business-warning li {
            margin-bottom: 4px;
        }

        /* 命令展示区域样式 */
        .commands-section {
            margin-top: 20px;
            background: white;
            border-radius: 8px;
            padding: 15px;
        }
        .commands-section h5 { color: #333; margin-bottom: 15px; }
        .command-item {
            background: #1e1e1e;
            border-radius: 8px;
            margin-bottom: 15px;
            overflow: hidden;
        }
        .command-header {
            padding: 10px 15px;
            background: #2d2d2d;
            color: #ccc;
            font-size: 0.85em;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .command-desc { color: #aaa; }
        .copy-btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 4px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.8em;
        }
        .copy-btn:hover { background: #5a6fd6; }
        .copy-btn.copied { background: #28a745; }
        .command-code {
            padding: 15px;
            color: #d4d4d4;
            font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
            font-size: 0.9em;
            overflow-x: auto;
        }
        .command-hint {
            padding: 10px 15px;
            background: #252526;
            color: #888;
            font-size: 0.8em;
            border-top: 1px solid #3c3c3c;
        }

        /* 建议样式 */
        .suggestions-section {
            margin-top: 20px;
            background: white;
            border-radius: 8px;
            padding: 15px;
        }
        .suggestions-section h5 { color: #333; margin-bottom: 15px; }
        .suggestion-group { margin-bottom: 15px; }
        .suggestion-group h6 {
            color: #667eea;
            font-size: 0.9em;
            margin-bottom: 8px;
            padding-left: 10px;
            border-left: 3px solid #667eea;
        }
        .suggestion-group.long-term h6 {
            color: #6c757d;
            border-left-color: #6c757d;
        }
        .suggestion-item {
            padding: 8px 15px;
            background: #f8f9fa;
            border-radius: 4px;
            margin-bottom: 5px;
            font-size: 0.9em;
            color: #555;
        }

        /* 可折叠组件样式 */
        details.hot-path-details { margin-bottom: 15px; }
        details.hot-path-details summary {
            list-style: none;
            cursor: pointer;
        }
        details.hot-path-details summary::-webkit-details-marker { display: none; }
        details.hot-path-details[open] .hot-path-header::after { content: "▼"; }
        details.hot-path-details:not([open]) .hot-path-header::after { content: "▶"; }
        .hot-path-header::after {
            margin-left: 10px;
            font-size: 0.8em;
        }

        /* 趋势图表样式 */
        .trend-chart {
            background: white;
            border-radius: 8px;
            padding: 15px;
            margin-top: 15px;
        }
        .trend-chart h5 {
            color: #333;
            margin-bottom: 10px;
            font-size: 0.9em;
        }
        .chart-container {
            position: relative;
            height: 150px;
            background: #f8f9fa;
            border-radius: 8px;
            padding: 10px;
        }
        .chart-svg {
            width: 100%;
            height: 100%;
        }
        .chart-line {
            fill: none;
            stroke: #667eea;
            stroke-width: 2;
            stroke-linecap: round;
            stroke-linejoin: round;
        }
        .chart-area {
            fill: url(#chartGradient);
            opacity: 0.3;
        }
        .chart-point {
            fill: #667eea;
            stroke: white;
            stroke-width: 2;
        }
        .chart-point:hover {
            fill: #764ba2;
            r: 6;
        }
        .chart-grid-line {
            stroke: #e9ecef;
            stroke-width: 1;
        }
        .chart-axis-label {
            font-size: 10px;
            fill: #888;
        }
        .chart-tooltip {
            position: absolute;
            background: #333;
            color: white;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.2s;
            white-space: nowrap;
        }
        .chart-legend {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin-top: 10px;
            font-size: 0.8em;
            color: #666;
        }
        .chart-legend-item {
            display: flex;
            align-items: center;
            gap: 5px;
        }
        .chart-legend-color {
            width: 12px;
            height: 3px;
            background: #667eea;
            border-radius: 2px;
        }
        .chart-legend-color.increasing { background: #dc3545; }
        .chart-legend-color.decreasing { background: #28a745; }
        .chart-legend-color.stable { background: #6c757d; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 {{.Title}}</h1>
            <div class="version">{{.Version}}</div>
            <div class="generated">生成时间: {{.Generated}}</div>
        </div>

        {{if .Findings}}
        <div class="findings">
            <div class="findings-header">
                <span class="group-icon">🚨</span>
                <span class="group-title">问题发现</span>
                <span class="group-count">{{len .Findings}} 个发现</span>
            </div>

            {{range .Findings}}
            <div class="finding-item finding-{{.Severity}}">
                <div class="finding-title">{{.Title}}</div>
                <div class="finding-meta">
                    规则: {{.RuleName}} ({{.RuleID}}) | 严重程度: {{.Severity}}
                </div>

                {{$ctx := index $.ProblemContexts .RuleID}}
                {{if $ctx}}
                <div class="problem-context">
                    {{if $ctx.Explanation}}
                    <div class="problem-explanation">
                        <h5>📝 问题解释</h5>
                        <p>{{$ctx.Explanation}}</p>
                    </div>
                    {{end}}

                    {{if $ctx.Impact}}
                    <div class="problem-impact">
                        <h5>📊 影响评估</h5>
                        <p>{{$ctx.Impact}}</p>
                    </div>
                    {{end}}

                    {{if $ctx.HotPaths}}
                    <div class="hot-paths">
                        <h5>🔥 热点调用链</h5>
                        {{range $idx, $hp := $ctx.HotPaths}}
                        <details class="hot-path-details" {{if eq $idx 0}}open{{end}}>
                            <summary>
                                <div class="hot-path-item">
                                    <div class="hot-path-header">
                                        <span class="hot-path-title">热点 #{{$hp.Index}}</span>
                                        <span class="hot-path-pct">{{printf "%.1f" $hp.TotalPct}}%</span>
                                    </div>
                                </div>
                            </summary>
                            <div class="hot-path-summary">调用链: {{$hp.Summary}}</div>
                            <div class="call-chain">
                                {{range $hp.Frames}}
                                {{if .IsNewSection}}
                                <div class="section-divider">─────────────────────────────</div>
                                {{end}}
                                <div class="call-chain-frame {{if .IsHighlight}}highlight{{end}}">
                                    <span class="frame-category frame-{{.Category}}">{{.CategoryIcon}} {{.Category}}</span>
                                    <div class="frame-info">
                                        <div class="frame-name">{{.ShortName}}</div>
                                        <div class="frame-location">
                                            {{if .FileLink}}
                                            <a href="{{.FileLink}}">{{.Location}}</a>
                                            {{else}}
                                            {{.Location}}
                                            {{end}}
                                        </div>
                                    </div>
                                    {{if .HighlightTag}}
                                    <span class="frame-tag {{if eq .HighlightTag "根因"}}root-cause{{end}}">← {{.HighlightTag}}</span>
                                    {{end}}
                                </div>
                                {{end}}
                                {{if not $hp.HasBusiness}}
                                <div class="no-business-warning">
                                    <strong>⚠️ 该路径中没有业务代码</strong>
                                    <p style="margin: 8px 0 0 0; font-size: 0.9em;">
                                        这可能意味着：
                                        <ul style="margin: 5px 0 0 15px; padding: 0;">
                                            <li><strong>运行时/GC 开销</strong>：Go 运行时或垃圾回收器消耗的资源，通常是正常的系统开销</li>
                                            <li><strong>标准库调用</strong>：业务代码通过标准库间接触发的操作（如 I/O、网络、JSON 解析等）</li>
                                            <li><strong>第三方库内部</strong>：第三方依赖库的内部实现消耗</li>
                                        </ul>
                                    </p>
                                    <p style="margin: 8px 0 0 0; font-size: 0.85em; color: #666;">
                                        💡 <strong>建议</strong>：查看调用链中的标准库/第三方库函数，追溯是哪个业务代码触发了这些调用。
                                        如果是 GC 相关，考虑减少内存分配或使用对象池。
                                    </p>
                                </div>
                                {{end}}
                            </div>
                        </details>
                        {{end}}
                    </div>
                    {{end}}

                    {{if $ctx.Commands}}
                    <details class="commands-details">
                        <summary class="commands-summary">💻 调试命令 (点击展开)</summary>
                        <div class="commands-section">
                            {{range $idx, $cmd := $ctx.Commands}}
                            <div class="command-item">
                                <div class="command-header">
                                    <span class="command-desc">{{$cmd.Index}}. {{$cmd.Description}}</span>
                                    <button class="copy-btn" onclick="copyCommand(this, '{{escapeJS $cmd.Command}}')">复制</button>
                                </div>
                                <div class="command-code">$ {{$cmd.Command}}</div>
                                {{if $cmd.OutputHint}}
                                <div class="command-hint">说明: {{$cmd.OutputHint}}</div>
                                {{end}}
                            </div>
                            {{end}}
                        </div>
                    </details>
                    {{end}}

                    {{if or $ctx.ImmediateSuggestions $ctx.LongTermSuggestions}}
                    <div class="suggestions-section">
                        <h5>💡 优化建议</h5>
                        {{if $ctx.ImmediateSuggestions}}
                        <div class="suggestion-group immediate">
                            <h6>🚀 立即可行</h6>
                            {{range $ctx.ImmediateSuggestions}}
                            <div class="suggestion-item">{{.Content}}</div>
                            {{end}}
                        </div>
                        {{end}}
                        {{if $ctx.LongTermSuggestions}}
                        <div class="suggestion-group long-term">
                            <h6>📋 长期改进</h6>
                            {{range $ctx.LongTermSuggestions}}
                            <div class="suggestion-item">{{.Content}}</div>
                            {{end}}
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        {{range .Groups}}
        <div class="group">
            <div class="group-header">
                <span class="group-icon">{{if eq .Type "cpu"}}⚡{{else if eq .Type "heap"}}💾{{else if eq .Type "goroutine"}}🔄{{else}}📁{{end}}</span>
                <span class="group-title">{{.Type}} 分析</span>
                <span class="group-count">{{len .Files}} 个文件</span>
            </div>

            {{range $index, $file := .Files}}
            <div class="file-card">
                <div class="file-header">
                    <span class="file-number">{{add $index 1}}</span>
                    <span class="file-name">{{$file.Name}}</span>
                </div>
                <div class="file-meta">
                    <span>🕐 {{$file.Time}}</span>
                    <span>📦 {{$file.Size}}</span>
                </div>

                {{if $file.Metrics}}
                <div class="metrics-grid">
                    {{if eq $file.ProfileType "cpu"}}
                    {{if gt $file.Metrics.CPUTime 0}}
                    <div class="metric-card">
                        <div class="metric-label">CPU 时间</div>
                        <div class="metric-value highlight">{{$file.Metrics.CPUTime}}</div>
                    </div>
                    {{end}}
                    {{if gt $file.Metrics.Duration 0}}
                    <div class="metric-card">
                        <div class="metric-label">采样时长</div>
                        <div class="metric-value">{{$file.Metrics.Duration}}</div>
                    </div>
                    {{end}}
                    <div class="metric-card">
                        <div class="metric-label">样本数</div>
                        <div class="metric-value">{{$file.Metrics.TotalSamples}}</div>
                    </div>
                    {{else if eq $file.ProfileType "heap"}}
                    <div class="metric-card">
                        <div class="metric-label">已分配内存</div>
                        <div class="metric-value highlight">{{formatBytes $file.Metrics.AllocSpace}}</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-label">已分配对象</div>
                        <div class="metric-value">{{$file.Metrics.AllocObjects}}</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-label">使用中内存</div>
                        <div class="metric-value highlight">{{formatBytes $file.Metrics.InuseSpace}}</div>
                    </div>
                    <div class="metric-card">
                        <div class="metric-label">使用中对象</div>
                        <div class="metric-value">{{$file.Metrics.InuseObjects}}</div>
                    </div>
                    {{if gt $file.Metrics.AllocSpace 0}}
                    <div class="metric-card">
                        <div class="metric-label">GC 回收率</div>
                        <div class="metric-value highlight">{{printf "%.1f" (mul (div (sub $file.Metrics.AllocSpace $file.Metrics.InuseSpace) $file.Metrics.AllocSpace) 100)}}%</div>
                    </div>
                    {{end}}
                    {{else if eq $file.ProfileType "goroutine"}}
                    <div class="metric-card">
                        <div class="metric-label">Goroutine 数量</div>
                        <div class="metric-value highlight">{{$file.Metrics.GoroutineCount}}</div>
                    </div>
                    {{end}}
                </div>

                {{if $file.Metrics.TopFunctions}}
                <div class="top-functions">
                    <h4>Top {{if eq $file.ProfileType "heap"}}当前内存占用 (inuse_space){{else if eq $file.ProfileType "goroutine"}}调用路径{{else}}热点函数{{end}}</h4>
                    {{range $i, $fn := $file.Metrics.TopFunctions}}
                    {{if lt $i 5}}
                    {{if or (ne $file.ProfileType "heap") (gt $fn.Flat 0)}}
                    <div class="func-item">
                        <span class="func-rank {{if eq $i 0}}top1{{else if eq $i 1}}top2{{else if eq $i 2}}top3{{end}}">{{add $i 1}}</span>
                        <span class="func-name" title="{{$fn.Name}}">{{$fn.Name}}</span>
                        {{if eq $file.ProfileType "heap"}}
                        <span class="func-pct">{{printf "%.1f" $fn.FlatPct}}% ({{formatBytes $fn.Flat}})</span>
                        {{else if eq $file.ProfileType "goroutine"}}
                        <span class="func-pct">{{printf "%.1f" $fn.CumPct}}%</span>
                        {{else}}
                        <span class="func-pct">{{printf "%.1f" $fn.FlatPct}}%</span>
                        {{end}}
                    </div>
                    {{end}}
                    {{end}}
                    {{end}}
                </div>
                {{end}}
                
                {{if and (eq $file.ProfileType "heap") $file.Metrics.TopAllocFunctions}}
                <div class="top-functions">
                    <h4>Top 累计内存分配 (alloc_space)</h4>
                    {{range $i, $fn := $file.Metrics.TopAllocFunctions}}
                    {{if lt $i 5}}
                    {{if gt $fn.Flat 0}}
                    <div class="func-item">
                        <span class="func-rank {{if eq $i 0}}top1{{else if eq $i 1}}top2{{else if eq $i 2}}top3{{end}}">{{add $i 1}}</span>
                        <span class="func-name" title="{{$fn.Name}}">{{$fn.Name}}</span>
                        <span class="func-pct">{{printf "%.1f" $fn.FlatPct}}% ({{formatBytes $fn.Flat}})</span>
                    </div>
                    {{end}}
                    {{end}}
                    {{end}}
                </div>
                {{end}}
                {{end}}
            </div>
            {{end}}
            
            {{if .Insights}}
            <div class="insights-section">
                <h3>💡 关键发现</h3>
                {{range .Insights}}
                <div class="insight-card {{.Level}}">
                    <div class="insight-header">
                        <span class="insight-icon">
                            {{if eq .Level "critical"}}🔴{{else if eq .Level "warning"}}🟡{{else}}🔵{{end}}
                        </span>
                        <span class="insight-title">{{.Title}}</span>
                    </div>
                    <div class="insight-description">{{.Description}}</div>
                </div>
                {{end}}
            </div>
            {{end}}

            {{if .TimeRange}}
            <div class="stats">
                <div class="stat-item">
                    <span class="stat-icon">📊</span>
                    <span class="stat-label">时间范围:</span>
                    <span class="stat-value">{{.TimeRange}}</span>
                </div>
                <div class="stat-item">
                    <span class="stat-icon">⏱️</span>
                    <span class="stat-label">持续时间:</span>
                    <span class="stat-value">{{.Duration}}</span>
                </div>
            </div>
            {{end}}

            {{if .HasTrends}}
            <div class="trends">
                <h4>📈 趋势分析</h4>
                {{if and .Trends .Trends.HeapInuse}}
                {{if gt .Trends.HeapInuse.R2 0.7}}
                <div class="trend-item">
                    <span class="trend-icon">{{if eq .Trends.HeapInuse.Direction "increasing"}}📈{{else if eq .Trends.HeapInuse.Direction "decreasing"}}📉{{else}}➡️{{end}}</span>
                    <div class="trend-details">
                        <div class="trend-label">堆内存趋势: {{if eq .Trends.HeapInuse.Direction "increasing"}}持续增长 ⚠️{{else if eq .Trends.HeapInuse.Direction "decreasing"}}下降中{{else}}稳定{{end}}</div>
                        <div class="trend-stats">变化率: {{printf "%.2f" .Trends.HeapInuse.Slope}} bytes/采样 | 置信度: {{printf "%.0f" (mul .Trends.HeapInuse.R2 100)}}%</div>
                    </div>
                </div>
                {{end}}
                {{end}}
                {{if and .Trends .Trends.GoroutineCount}}
                {{if gt .Trends.GoroutineCount.R2 0.7}}
                <div class="trend-item">
                    <span class="trend-icon">{{if eq .Trends.GoroutineCount.Direction "increasing"}}📈{{else if eq .Trends.GoroutineCount.Direction "decreasing"}}📉{{else}}➡️{{end}}</span>
                    <div class="trend-details">
                        <div class="trend-label">Goroutine 趋势: {{if eq .Trends.GoroutineCount.Direction "increasing"}}持续增长 ⚠️{{else if eq .Trends.GoroutineCount.Direction "decreasing"}}下降中{{else}}稳定{{end}}</div>
                        <div class="trend-stats">变化率: {{printf "%.2f" .Trends.GoroutineCount.Slope}}/采样 | 置信度: {{printf "%.0f" (mul .Trends.GoroutineCount.R2 100)}}%</div>
                    </div>
                </div>
                {{end}}
                {{end}}

                {{if .ChartData}}
                <div class="trend-chart">
                    <h5>📊 {{.ChartUnit}}变化趋势图</h5>
                    <div class="chart-container">
                        <svg class="chart-svg" viewBox="0 0 400 120" preserveAspectRatio="xMidYMid meet">
                            <defs>
                                <linearGradient id="chartGradient-{{.Type}}" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#667eea;stop-opacity:0.4" />
                                    <stop offset="100%" style="stop-color:#667eea;stop-opacity:0.05" />
                                </linearGradient>
                            </defs>
                            <!-- 网格线 -->
                            <line class="chart-grid-line" x1="40" y1="10" x2="390" y2="10"/>
                            <line class="chart-grid-line" x1="40" y1="35" x2="390" y2="35"/>
                            <line class="chart-grid-line" x1="40" y1="60" x2="390" y2="60"/>
                            <line class="chart-grid-line" x1="40" y1="85" x2="390" y2="85"/>
                            <line class="chart-grid-line" x1="40" y1="110" x2="390" y2="110"/>
                            <!-- Y轴标签 -->
                            <text class="chart-axis-label" x="35" y="14" text-anchor="end">max</text>
                            <text class="chart-axis-label" x="35" y="114" text-anchor="end">min</text>
                            <!-- 数据折线和点通过 JavaScript 渲染 -->
                        </svg>
                        <script>
                        (function() {
                            var data = [{{range $i, $p := .ChartData}}{{if $i}},{{end}}{x:{{$p.Index}},y:{{$p.Normalized}},label:"{{$p.Label}}",time:"{{$p.Time}}"}{{end}}];
                            var svg = document.currentScript.previousElementSibling;
                            var n = data.length;
                            if (n < 2) return;
                            var step = 350 / (n - 1);
                            
                            // 绘制区域填充
                            var areaPath = "M ";
                            for (var i = 0; i < n; i++) {
                                var x = 40 + i * step;
                                var y = 110 - data[i].y;
                                areaPath += (i === 0 ? "" : " L ") + x + " " + y;
                            }
                            areaPath += " L " + (40 + (n-1) * step) + " 110 L 40 110 Z";
                            var area = document.createElementNS("http://www.w3.org/2000/svg", "path");
                            area.setAttribute("class", "chart-area");
                            area.setAttribute("d", areaPath);
                            area.setAttribute("style", "fill:url(#chartGradient-{{.Type}})");
                            svg.appendChild(area);
                            
                            // 绘制折线
                            var points = "";
                            for (var i = 0; i < n; i++) {
                                var x = 40 + i * step;
                                var y = 110 - data[i].y;
                                points += x + "," + y + " ";
                            }
                            var line = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
                            line.setAttribute("class", "chart-line");
                            line.setAttribute("points", points.trim());
                            svg.appendChild(line);
                            
                            // 绘制数据点
                            for (var i = 0; i < n; i++) {
                                var x = 40 + i * step;
                                var y = 110 - data[i].y;
                                var circle = document.createElementNS("http://www.w3.org/2000/svg", "circle");
                                circle.setAttribute("class", "chart-point");
                                circle.setAttribute("cx", x);
                                circle.setAttribute("cy", y);
                                circle.setAttribute("r", 4);
                                var title = document.createElementNS("http://www.w3.org/2000/svg", "title");
                                title.textContent = data[i].time + ": " + data[i].label;
                                circle.appendChild(title);
                                svg.appendChild(circle);
                            }
                            
                            // 绘制 X 轴时间标签
                            var firstLabel = document.createElementNS("http://www.w3.org/2000/svg", "text");
                            firstLabel.setAttribute("class", "chart-axis-label");
                            firstLabel.setAttribute("x", 40);
                            firstLabel.setAttribute("y", 120);
                            firstLabel.setAttribute("text-anchor", "start");
                            firstLabel.textContent = data[0].time;
                            svg.appendChild(firstLabel);
                            
                            var lastLabel = document.createElementNS("http://www.w3.org/2000/svg", "text");
                            lastLabel.setAttribute("class", "chart-axis-label");
                            lastLabel.setAttribute("x", 40 + (n-1) * step);
                            lastLabel.setAttribute("y", 120);
                            lastLabel.setAttribute("text-anchor", "end");
                            lastLabel.textContent = data[n-1].time;
                            svg.appendChild(lastLabel);
                        })();
                        </script>
                    </div>
                    <div class="chart-legend">
                        <div class="chart-legend-item">
                            <span class="chart-legend-color {{if and .Trends .Trends.HeapInuse}}{{.Trends.HeapInuse.Direction}}{{else if and .Trends .Trends.GoroutineCount}}{{.Trends.GoroutineCount.Direction}}{{end}}"></span>
                            <span>{{.ChartUnit}}使用量</span>
                        </div>
                        <div class="chart-legend-item">
                            <span style="color: #888;">首次: {{(index .ChartData 0).Label}}</span>
                        </div>
                        <div class="chart-legend-item">
                            <span style="color: #888;">最新: {{(index .ChartData (sub (len .ChartData) 1)).Label}}</span>
                        </div>
                    </div>
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}
    </div>

    <script>
    function copyCommand(btn, command) {
        navigator.clipboard.writeText(command).then(function() {
            btn.textContent = '已复制';
            btn.classList.add('copied');
            setTimeout(function() {
                btn.textContent = '复制';
                btn.classList.remove('copied');
            }, 2000);
        }).catch(function(err) {
            console.error('复制失败:', err);
        });
    }

    function copyCode(btn, idx) {
        var codeElement = document.getElementById('code-' + idx);
        var code = codeElement.textContent;
        navigator.clipboard.writeText(code).then(function() {
            btn.textContent = '已复制';
            btn.classList.add('copied');
            setTimeout(function() {
                btn.textContent = '复制代码';
                btn.classList.remove('copied');
            }, 2000);
        }).catch(function(err) {
            console.error('复制失败:', err);
        });
    }
    </script>
</body>
</html>`

// GenerateHTMLReport 生成 HTML 格式的分析报告（向后兼容）
func GenerateHTMLReport(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends, findings []rules.Finding, outputPath string) error {
	return GenerateHTMLReportWithContext(groups, trends, findings, nil, outputPath)
}

// GenerateHTMLReportWithContext 生成带问题上下文的 HTML 格式分析报告
func GenerateHTMLReportWithContext(groups []analyzer.ProfileGroup, trends map[string]*analyzer.GroupTrends, findings []rules.Finding, contexts map[string]*locator.ProblemContext, outputPath string) error {
	data := HTMLReportData{
		Title:           "PerfInspector 分析报告",
		Version:         "v0.1",
		Generated:       time.Now().UTC().Format(time.RFC3339),
		Findings:        findings,
		ProblemContexts: make(map[string]*HTMLProblemContext),
	}

	// 转换 ProblemContexts 为 HTML 友好格式
	for ruleID, ctx := range contexts {
		data.ProblemContexts[ruleID] = convertProblemContextToHTML(ctx)
	}

	for _, group := range groups {
		if len(group.Files) == 0 {
			continue
		}

		htmlGroup := HTMLGroupData{
			Type: group.Type,
		}

		for _, file := range group.Files {
			htmlGroup.Files = append(htmlGroup.Files, HTMLFileData{
				Name:        filepath.Base(file.Path),
				Time:        file.Time.UTC().Format(time.RFC3339),
				Size:        formatSize(file.Size),
				Metrics:     file.Metrics,
				ProfileType: group.Type,
			})
		}

		if len(group.Files) > 1 {
			first := group.Files[0].Time.UTC()
			last := group.Files[len(group.Files)-1].Time.UTC()
			duration := last.Sub(first)
			htmlGroup.TimeRange = fmt.Sprintf("%s → %s",
				first.Format("2006-01-02 15:04:05"),
				last.Format("2006-01-02 15:04:05"))
			htmlGroup.Duration = formatDuration(duration)
		}

		if groupTrends, ok := trends[group.Type]; ok && groupTrends != nil {
			htmlGroup.Trends = groupTrends
			if (groupTrends.HeapInuse != nil && groupTrends.HeapInuse.R2 > 0.7) ||
				(groupTrends.GoroutineCount != nil && groupTrends.GoroutineCount.R2 > 0.7) {
				htmlGroup.HasTrends = true

				// 生成图表数据点
				htmlGroup.ChartData, htmlGroup.ChartType, htmlGroup.ChartUnit, htmlGroup.ChartMax, htmlGroup.ChartMin = generateChartData(group)
			}
		}

		// 对于 heap profile，生成智能洞察
		if group.Type == "heap" && len(group.Files) > 0 && group.Files[0].Metrics != nil {
			htmlGroup.Insights = analyzer.AnalyzeHeapInsights(group.Files[0].Metrics)
		}

		data.Groups = append(data.Groups, htmlGroup)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b interface{}) interface{} {
			switch va := a.(type) {
			case int:
				if vb, ok := b.(int); ok {
					return va - vb
				}
			case int64:
				if vb, ok := b.(int64); ok {
					return va - vb
				}
			}
			return 0
		},
		"mul": func(a interface{}, b float64) float64 {
			switch v := a.(type) {
			case int:
				return float64(v) * b
			case int64:
				return float64(v) * b
			case float64:
				return v * b
			default:
				return 0
			}
		},
		"div": func(a, b interface{}) float64 {
			var fa, fb float64
			switch v := a.(type) {
			case int:
				fa = float64(v)
			case int64:
				fa = float64(v)
			case float64:
				fa = v
			}
			switch v := b.(type) {
			case int:
				fb = float64(v)
			case int64:
				fb = float64(v)
			case float64:
				fb = v
			}
			if fb == 0 {
				return 0
			}
			return fa / fb
		},
		"formatBytes": analyzer.FormatBytes,
		"escapeJS":    escapeJSString,
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outputPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// convertProblemContextToHTML 转换 ProblemContext 为 HTML 模板友好格式
func convertProblemContextToHTML(ctx *locator.ProblemContext) *HTMLProblemContext {
	if ctx == nil {
		return nil
	}

	htmlCtx := &HTMLProblemContext{
		Title:       ctx.Title,
		Severity:    ctx.Severity,
		Explanation: ctx.Explanation,
		Impact:      ctx.Impact,
		HotPaths:    ConvertHotPathsForHTML(ctx.HotPaths),
		Commands:    ConvertCommandsForHTML(ctx.Commands),
	}

	// 分离立即和长期建议
	htmlCtx.ImmediateSuggestions, htmlCtx.LongTermSuggestions = ConvertSuggestionsForHTML(ctx.Suggestions)

	return htmlCtx
}

// escapeJSString 转义 JavaScript 字符串
func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// ConvertHotPathsForHTML 将 HotPath 列表转换为 HTML 友好格式
func ConvertHotPathsForHTML(hotPaths []locator.HotPath) []HTMLHotPath {
	result := make([]HTMLHotPath, 0, len(hotPaths))
	for i, hp := range hotPaths {
		htmlHP := HTMLHotPath{
			Index:          i + 1,
			TotalPct:       hp.Chain.TotalPct,
			Summary:        hp.Chain.Summary(),
			HasBusiness:    hp.Chain.HasBusinessCode(),
			RootCauseIndex: hp.RootCauseIndex,
		}

		// 创建业务帧索引集合
		businessFrameSet := make(map[int]bool)
		for _, idx := range hp.BusinessFrames {
			businessFrameSet[idx] = true
		}

		// 转换栈帧
		var lastCategory locator.CodeCategory
		for j, frame := range hp.Chain.Frames {
			htmlFrame := HTMLStackFrame{
				Index:        j,
				Category:     string(frame.Category),
				CategoryIcon: frame.Category.Icon(),
				ShortName:    frame.ShortName,
				Location:     frame.Location(),
				FileLink:     template.URL(generateFileLink(frame.FilePath, frame.LineNumber)),
				IsHighlight:  businessFrameSet[j],
				IsNewSection: j > 0 && frame.Category != lastCategory,
			}

			// 设置高亮标签
			if businessFrameSet[j] {
				if j == hp.RootCauseIndex {
					htmlFrame.HighlightTag = "根因"
				} else {
					htmlFrame.HighlightTag = "关注"
				}
			}

			htmlHP.Frames = append(htmlHP.Frames, htmlFrame)
			lastCategory = frame.Category
		}

		result = append(result, htmlHP)
	}
	return result
}

// ConvertCommandsForHTML 将命令列表转换为 HTML 友好格式
func ConvertCommandsForHTML(commands []locator.ExecutableCmd) []HTMLExecutableCmd {
	result := make([]HTMLExecutableCmd, 0, len(commands))
	for i, cmd := range commands {
		result = append(result, HTMLExecutableCmd{
			Index:       i + 1,
			Command:     cmd.Command,
			Description: cmd.Description,
			OutputHint:  cmd.OutputHint,
		})
	}
	return result
}

// ConvertSuggestionsForHTML 将建议列表转换为 HTML 友好格式，分离立即和长期建议
func ConvertSuggestionsForHTML(suggestions []locator.Suggestion) (immediate, longTerm []HTMLSuggestion) {
	for _, s := range suggestions {
		htmlSuggestion := HTMLSuggestion{
			Category: s.Category,
			Content:  s.Content,
		}
		if s.Category == "long_term" {
			longTerm = append(longTerm, htmlSuggestion)
		} else {
			immediate = append(immediate, htmlSuggestion)
		}
	}
	return
}

// generateFileLink 生成 file:// 协议链接
func generateFileLink(filePath string, lineNumber int64) string {
	if filePath == "" || filePath == "unknown" {
		return ""
	}
	// 对于本地文件，生成 file:// 链接
	// 注意：这在大多数浏览器中可能不会直接打开，但提供了路径信息
	if strings.HasPrefix(filePath, "/") {
		if lineNumber > 0 {
			return fmt.Sprintf("file://%s#L%d", filePath, lineNumber)
		}
		return fmt.Sprintf("file://%s", filePath)
	}
	return ""
}

// GetCategoryClass 返回类别对应的 CSS 类名
func GetCategoryClass(category locator.CodeCategory) string {
	switch category {
	case locator.CategoryRuntime:
		return "frame-runtime"
	case locator.CategoryStdlib:
		return "frame-stdlib"
	case locator.CategoryThirdParty:
		return "frame-third-party"
	case locator.CategoryBusiness:
		return "frame-business"
	default:
		return "frame-unknown"
	}
}

// generateChartData 从 ProfileGroup 生成图表数据点
func generateChartData(group analyzer.ProfileGroup) ([]HTMLChartPoint, string, string, float64, float64) {
	if len(group.Files) < 2 {
		return nil, "", "", 0, 0
	}

	var points []HTMLChartPoint
	var chartType, chartUnit string
	var minVal, maxVal float64

	switch group.Type {
	case "heap":
		chartType = "heap"
		chartUnit = "内存"
		// 提取堆内存数据
		for i, file := range group.Files {
			if file.Metrics != nil {
				val := float64(file.Metrics.InuseSpace)
				if i == 0 || val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
				points = append(points, HTMLChartPoint{
					Index: i,
					Value: val,
					Label: analyzer.FormatBytes(file.Metrics.InuseSpace),
					Time:  file.Time.UTC().Format("15:04:05"),
				})
			}
		}

	case "goroutine":
		chartType = "goroutine"
		chartUnit = "Goroutine"
		// 提取 goroutine 数量
		for i, file := range group.Files {
			if file.Metrics != nil {
				val := float64(file.Metrics.GoroutineCount)
				if i == 0 || val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
				points = append(points, HTMLChartPoint{
					Index: i,
					Value: val,
					Label: fmt.Sprintf("%d", file.Metrics.GoroutineCount),
					Time:  file.Time.UTC().Format("15:04:05"),
				})
			}
		}
	}

	// 归一化数据点 (0-100)
	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1 // 避免除零
	}
	for i := range points {
		points[i].Normalized = ((points[i].Value - minVal) / valueRange) * 100
	}

	return points, chartType, chartUnit, maxVal, minVal
}
