package analyzer

import (
	"math"
)

// TrendMetrics 趋势指标
type TrendMetrics struct {
	Slope     float64 // 斜率
	R2        float64 // R² 决定系数
	Direction string  // "increasing", "decreasing", "stable"
}

// GroupTrends 分组趋势数据
type GroupTrends struct {
	HeapInuse      *TrendMetrics // 堆内存使用趋势
	GoroutineCount *TrendMetrics // Goroutine 数量趋势
}

// CalculateTrends 计算 profile 组的趋势
// 需要至少 3 个文件才能计算趋势
func CalculateTrends(group ProfileGroup) *GroupTrends {
	if len(group.Files) < 3 {
		return nil
	}

	trends := &GroupTrends{}

	switch group.Type {
	case "heap":
		// 从 Metrics 中提取堆内存数据点
		var heapValues []float64
		for _, file := range group.Files {
			if file.Metrics != nil {
				heapValues = append(heapValues, float64(file.Metrics.InuseSpace))
			}
		}
		if len(heapValues) >= 3 {
			slope, r2 := LinearRegression(heapValues)
			trends.HeapInuse = &TrendMetrics{
				Slope:     slope,
				R2:        r2,
				Direction: getDirection(slope),
			}
		}

	case "goroutine":
		// 从 Metrics 中提取 goroutine 数量数据点
		var goroutineValues []float64
		for _, file := range group.Files {
			if file.Metrics != nil {
				goroutineValues = append(goroutineValues, float64(file.Metrics.GoroutineCount))
			}
		}
		if len(goroutineValues) >= 3 {
			slope, r2 := LinearRegression(goroutineValues)
			trends.GoroutineCount = &TrendMetrics{
				Slope:     slope,
				R2:        r2,
				Direction: getDirection(slope),
			}
		}
	}

	return trends
}

// LinearRegression 计算线性回归的斜率和 R²
// 使用最小二乘法
func LinearRegression(values []float64) (slope, r2 float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, 0
	}

	// 检查是否有无效值
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0
		}
	}

	// 计算均值
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	meanX := sumX / n
	meanY := sumY / n

	// 检查均值是否有效
	if math.IsNaN(meanY) || math.IsInf(meanY, 0) {
		return 0, 0
	}

	// 计算斜率
	numerator := sumXY - n*meanX*meanY
	denominator := sumX2 - n*meanX*meanX

	if denominator == 0 {
		return 0, 0
	}

	slope = numerator / denominator

	// 检查斜率是否有效
	if math.IsNaN(slope) || math.IsInf(slope, 0) {
		return 0, 0
	}

	// 计算 R²
	// R² = 1 - SS_res / SS_tot
	var ssRes, ssTot float64
	intercept := meanY - slope*meanX

	// 检查截距是否有效
	if math.IsNaN(intercept) || math.IsInf(intercept, 0) {
		return 0, 0
	}

	for i, y := range values {
		x := float64(i)
		predicted := slope*x + intercept
		if math.IsNaN(predicted) || math.IsInf(predicted, 0) {
			return 0, 0
		}
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}

	// 检查 ssRes 和 ssTot 是否有效
	if math.IsNaN(ssRes) || math.IsInf(ssRes, 0) ||
		math.IsNaN(ssTot) || math.IsInf(ssTot, 0) {
		return 0, 0
	}

	if ssTot == 0 {
		// 所有值相同
		r2 = 1.0
	} else {
		r2 = 1 - ssRes/ssTot
	}

	// 确保 R² 在 [0, 1] 范围内
	if math.IsNaN(r2) || math.IsInf(r2, 0) {
		r2 = 0
	}
	r2 = math.Max(0, math.Min(1, r2))

	return slope, r2
}

// getDirection 根据斜率判断趋势方向
func getDirection(slope float64) string {
	const threshold = 0.01 // 斜率阈值
	if slope > threshold {
		return "increasing"
	} else if slope < -threshold {
		return "decreasing"
	}
	return "stable"
}
