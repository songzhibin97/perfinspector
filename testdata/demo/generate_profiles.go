//go:build ignore
// +build ignore

// 生成多个 pprof 文件用于测试 PerfInspector
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var globalData [][]byte

// CPU 密集型任务
func cpuWork(iterations int) {
	sum := 0
	for i := 0; i < iterations; i++ {
		sum += i * i % 1000
	}
}

// 内存分配任务
func memoryWork(size int) {
	globalData = append(globalData, make([]byte, size))
}

// 生成 CPU profile
func generateCPUProfile(filename string, workload int) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return err
	}

	cpuWork(workload)
	time.Sleep(50 * time.Millisecond)

	pprof.StopCPUProfile()
	return nil
}

// 生成 Heap profile
func generateHeapProfile(filename string, allocSize int) error {
	// 分配内存
	memoryWork(allocSize)
	runtime.GC()

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return pprof.WriteHeapProfile(f)
}

// 生成 Goroutine profile
func generateGoroutineProfile(filename string, numGoroutines int) error {
	// 创建一些 goroutine
	done := make(chan bool)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			time.Sleep(2 * time.Second)
			done <- true
		}()
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return pprof.Lookup("goroutine").WriteTo(f, 0)
}

func main() {
	// 确保目录存在
	os.MkdirAll("testdata/profiles", 0755)

	fmt.Println("生成 CPU profiles...")
	for i := 1; i <= 3; i++ {
		filename := fmt.Sprintf("testdata/profiles/cpu%d.pprof", i)
		workload := 10000000 * i
		if err := generateCPUProfile(filename, workload); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", filename, err)
		} else {
			fmt.Printf("✅ 生成 %s\n", filename)
		}
		time.Sleep(500 * time.Millisecond) // 确保时间戳不同
	}

	fmt.Println("\n生成 Heap profiles...")
	for i := 1; i <= 3; i++ {
		filename := fmt.Sprintf("testdata/profiles/heap%d.pprof", i)
		allocSize := 1024 * 1024 * i // 1MB, 2MB, 3MB
		if err := generateHeapProfile(filename, allocSize); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", filename, err)
		} else {
			fmt.Printf("✅ 生成 %s\n", filename)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n生成 Goroutine profile...")
	if err := generateGoroutineProfile("testdata/profiles/goroutine1.pprof", 10); err != nil {
		fmt.Printf("生成 goroutine profile 失败: %v\n", err)
	} else {
		fmt.Println("✅ 生成 testdata/profiles/goroutine1.pprof")
	}

	fmt.Println("\n所有 profile 文件生成完成！")
}
