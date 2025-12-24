//go:build ignore
// +build ignore

// 场景4：CPU 热点 - 某个函数消耗大量 CPU
// 模拟：低效的算法导致 CPU 使用率高
package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

// 低效的斐波那契实现（指数复杂度）
// 注意：这个函数故意使用低效的递归实现来产生CPU热点
func slowFib(n int) int {
	if n <= 1 {
		return n
	}
	return slowFib(n-1) + slowFib(n-2)
}

// 低效的字符串拼接
// 注意：这个函数故意使用低效的字符串拼接来产生CPU热点
func slowStringConcat(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += "x" // 每次都创建新字符串
		// 添加一些额外的CPU计算，确保能被采样到
		_ = len(result) * i
	}
	return result
}

// 低效的矩阵乘法 - 纯CPU密集型操作
func slowMatrixMultiply(size int) [][]int {
	// 创建两个矩阵
	a := make([][]int, size)
	b := make([][]int, size)
	c := make([][]int, size)
	for i := 0; i < size; i++ {
		a[i] = make([]int, size)
		b[i] = make([]int, size)
		c[i] = make([]int, size)
		for j := 0; j < size; j++ {
			a[i][j] = i + j
			b[i][j] = i * j
		}
	}

	// O(n^3) 矩阵乘法
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			sum := 0
			for k := 0; k < size; k++ {
				sum += a[i][k] * b[k][j]
			}
			c[i][j] = sum
		}
	}
	return c
}

// 低效的排序（冒泡排序）
func bubbleSort(arr []int) {
	n := len(arr)
	for i := 0; i < n; i++ {
		for j := 0; j < n-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}

func generateCPUProfile(filename string, workFunc func()) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return err
	}

	workFunc()

	pprof.StopCPUProfile()
	return nil
}

func main() {
	dir := "testdata/profiles/scenario4_cpu_hotspot"
	os.MkdirAll(dir, 0755)

	fmt.Println("场景4：CPU 热点（低效算法）")
	fmt.Println("模拟：多种低效算法导致 CPU 使用率高")

	// Profile 1: 低效斐波那契
	fmt.Println("\n生成 CPU Profile 1: 低效斐波那契...")
	cpuFile1 := fmt.Sprintf("%s/cpu_fib.pprof", dir)
	err := generateCPUProfile(cpuFile1, func() {
		// 增加计算量确保有足够的采样数据
		// slowFib(40) 的计算量是 slowFib(35) 的约30倍
		for i := 0; i < 5; i++ {
			slowFib(40) // 使用更大的n值，确保足够的CPU时间
		}
	})
	if err != nil {
		fmt.Printf("生成 %s 失败: %v\n", cpuFile1, err)
	} else {
		fmt.Printf("✅ %s\n", cpuFile1)
	}

	time.Sleep(500 * time.Millisecond)

	// Profile 2: 低效矩阵乘法
	fmt.Println("\n生成 CPU Profile 2: 低效矩阵乘法...")
	cpuFile2 := fmt.Sprintf("%s/cpu_matrix.pprof", dir)
	err = generateCPUProfile(cpuFile2, func() {
		// 使用矩阵乘法，纯CPU密集型操作
		for i := 0; i < 10; i++ {
			slowMatrixMultiply(300) // 300x300矩阵乘法
		}
	})
	if err != nil {
		fmt.Printf("生成 %s 失败: %v\n", cpuFile2, err)
	} else {
		fmt.Printf("✅ %s\n", cpuFile2)
	}

	time.Sleep(500 * time.Millisecond)

	// Profile 3: 低效排序
	fmt.Println("\n生成 CPU Profile 3: 冒泡排序...")
	cpuFile3 := fmt.Sprintf("%s/cpu_sort.pprof", dir)
	err = generateCPUProfile(cpuFile3, func() {
		// 增加计算量确保有足够的采样数据
		for i := 0; i < 50; i++ {
			arr := make([]int, 10000) // 增加数组大小
			for j := range arr {
				arr[j] = 10000 - j // 逆序，最坏情况
			}
			bubbleSort(arr)
		}
	})
	if err != nil {
		fmt.Printf("生成 %s 失败: %v\n", cpuFile3, err)
	} else {
		fmt.Printf("✅ %s\n", cpuFile3)
	}

	fmt.Println("\n预期结果：")
	fmt.Println("- 各 profile 显示不同的 CPU 热点函数")
	fmt.Println("- slowFib, slowMatrixMultiply, bubbleSort 应该是热点")
}
