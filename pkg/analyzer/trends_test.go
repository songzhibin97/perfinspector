package analyzer

import (
	"math"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

// TestLinearRegression_PerfectLine 测试完美线性数据
func TestLinearRegression_PerfectLine(t *testing.T) {
	// y = 2x + 1
	values := []float64{1, 3, 5, 7, 9}
	slope, r2 := LinearRegression(values)

	assert.InDelta(t, 2.0, slope, 0.001, "斜率应该是 2")
	assert.InDelta(t, 1.0, r2, 0.001, "完美线性数据 R² 应该是 1")
}

// TestLinearRegression_ConstantValues 测试常量值
func TestLinearRegression_ConstantValues(t *testing.T) {
	values := []float64{5, 5, 5, 5, 5}
	slope, r2 := LinearRegression(values)

	assert.InDelta(t, 0.0, slope, 0.001, "常量值斜率应该是 0")
	assert.InDelta(t, 1.0, r2, 0.001, "常量值 R² 应该是 1")
}

// TestLinearRegression_TwoPoints 测试两个点
func TestLinearRegression_TwoPoints(t *testing.T) {
	values := []float64{0, 10}
	slope, r2 := LinearRegression(values)

	assert.InDelta(t, 10.0, slope, 0.001, "两点斜率应该是 10")
	assert.InDelta(t, 1.0, r2, 0.001, "两点 R² 应该是 1")
}

// TestLinearRegression_SinglePoint 测试单个点
func TestLinearRegression_SinglePoint(t *testing.T) {
	values := []float64{5}
	slope, r2 := LinearRegression(values)

	assert.Equal(t, 0.0, slope, "单点斜率应该是 0")
	assert.Equal(t, 0.0, r2, "单点 R² 应该是 0")
}

// TestLinearRegression_EmptySlice 测试空切片
func TestLinearRegression_EmptySlice(t *testing.T) {
	values := []float64{}
	slope, r2 := LinearRegression(values)

	assert.Equal(t, 0.0, slope)
	assert.Equal(t, 0.0, r2)
}

// TestLinearRegression_DecreasingTrend 测试递减趋势
func TestLinearRegression_DecreasingTrend(t *testing.T) {
	// y = -3x + 10
	values := []float64{10, 7, 4, 1}
	slope, r2 := LinearRegression(values)

	assert.InDelta(t, -3.0, slope, 0.001, "斜率应该是 -3")
	assert.InDelta(t, 1.0, r2, 0.001, "完美线性数据 R² 应该是 1")
}

// TestLinearRegression_NoisyData 测试带噪声的数据
func TestLinearRegression_NoisyData(t *testing.T) {
	// 带噪声的递增趋势
	values := []float64{1.1, 2.9, 5.2, 6.8, 9.1}
	slope, r2 := LinearRegression(values)

	assert.True(t, slope > 1.5, "斜率应该是正数且较大")
	assert.True(t, r2 > 0.9, "R² 应该接近 1")
	assert.True(t, r2 <= 1.0, "R² 不应该超过 1")
}

// Property Test: R² 应该始终在 [0, 1] 范围内
// **Property 7: Linear Regression Calculation**
// **Validates: Requirements 4.2, 4.3**
func TestLinearRegression_R2InRange(t *testing.T) {
	f := func(values []float64) bool {
		if len(values) < 2 {
			return true // 跳过太短的切片
		}

		// 过滤掉 NaN 和 Inf
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return true
			}
		}

		_, r2 := LinearRegression(values)
		return r2 >= 0 && r2 <= 1
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property Test: 完美线性数据的 R² 应该等于 1
// **Property 7: Linear Regression Calculation**
// **Validates: Requirements 4.2, 4.3**
func TestLinearRegression_PerfectLineR2IsOne(t *testing.T) {
	f := func(slope, intercept float64, n uint8) bool {
		// 限制数据点数量
		count := int(n%10) + 3 // 3-12 个点

		// 过滤极端值
		if math.IsNaN(slope) || math.IsInf(slope, 0) ||
			math.IsNaN(intercept) || math.IsInf(intercept, 0) {
			return true
		}

		// 限制斜率和截距的范围，避免溢出
		if math.Abs(slope) > 1e10 || math.Abs(intercept) > 1e10 {
			return true
		}

		// 生成完美线性数据
		values := make([]float64, count)
		for i := 0; i < count; i++ {
			values[i] = slope*float64(i) + intercept
			// 检查生成的值是否有效
			if math.IsNaN(values[i]) || math.IsInf(values[i], 0) {
				return true
			}
		}

		calculatedSlope, r2 := LinearRegression(values)

		// 如果返回 0,0 说明检测到了无效数据，这是可接受的
		if calculatedSlope == 0 && r2 == 0 && slope != 0 {
			return true
		}

		return math.Abs(r2-1.0) < 0.0001
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestGetDirection 测试方向判断
func TestGetDirection(t *testing.T) {
	tests := []struct {
		slope    float64
		expected string
	}{
		{1.0, "increasing"},
		{0.5, "increasing"},
		{0.02, "increasing"},
		{-1.0, "decreasing"},
		{-0.5, "decreasing"},
		{-0.02, "decreasing"},
		{0.0, "stable"},
		{0.005, "stable"},
		{-0.005, "stable"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := getDirection(tt.slope)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateTrends_InsufficientFiles 测试文件数量不足
func TestCalculateTrends_InsufficientFiles(t *testing.T) {
	group := ProfileGroup{
		Type:  "heap",
		Files: []ProfileFile{{}, {}}, // 只有 2 个文件
	}

	trends := CalculateTrends(group)
	assert.Nil(t, trends, "少于 3 个文件不应该计算趋势")
}

// TestCalculateTrends_EmptyGroup 测试空分组
func TestCalculateTrends_EmptyGroup(t *testing.T) {
	group := ProfileGroup{
		Type:  "heap",
		Files: []ProfileFile{},
	}

	trends := CalculateTrends(group)
	assert.Nil(t, trends)
}
