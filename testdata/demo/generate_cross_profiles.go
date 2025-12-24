//go:build ignore
// +build ignore

// 生成用于测试联合分析的 pprof 文件
// 模拟 goroutine 泄漏导致内存增长的场景
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var (
	globalData  [][]byte
	leakedChans []chan struct{}
)

// 模拟 goroutine 泄漏：创建 goroutine 但不关闭 channel
func leakGoroutines(count int) {
	for i := 0; i < count; i++ {
		ch := make(chan struct{})
		leakedChans = append(leakedChans, ch)
		go func(c chan struct{}) {
			<-c // 永远阻塞，因为没人会关闭这个 channel
		}(ch)
	}
}

// 模拟内存分配
func allocateMemory(size int) {
	globalData = append(globalData, make([]byte, size))
}

// 生成 Heap profile
func generateHeapProfile(filename string) error {
	runtime.GC()
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.WriteHeapProfile(f)
}

// 生成 Goroutine profile
func generateGoroutineProfile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.Lookup("goroutine").WriteTo(f, 0)
}

func main() {
	dir := "testdata/profiles/cross"
	os.MkdirAll(dir, 0755)

	fmt.Println("生成联合分析测试数据...")
	fmt.Println("模拟场景：Goroutine 泄漏导致内存增长")

	// 生成 3 组数据，每组包含 heap 和 goroutine profile
	for i := 1; i <= 3; i++ {
		// 泄漏一些 goroutine（每次增加 10 个）
		leakGoroutines(10 * i)

		// 分配一些内存（每次增加 1MB）
		allocateMemory(1024 * 1024 * i)

		// 等待一下让 GC 稳定
		time.Sleep(100 * time.Millisecond)

		// 生成 heap profile
		heapFile := fmt.Sprintf("%s/heap_cross_%d.pprof", dir, i)
		if err := generateHeapProfile(heapFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", heapFile, err)
		} else {
			fmt.Printf("✅ 生成 %s (内存: %d MB, Goroutines: %d)\n",
				heapFile, i, runtime.NumGoroutine())
		}

		// 生成 goroutine profile
		goroutineFile := fmt.Sprintf("%s/goroutine_cross_%d.pprof", dir, i)
		if err := generateGoroutineProfile(goroutineFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", goroutineFile, err)
		} else {
			fmt.Printf("✅ 生成 %s\n", goroutineFile)
		}

		// 等待确保时间戳不同
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("\n总计泄漏 Goroutine: %d\n", len(leakedChans))
	fmt.Printf("总计分配内存: %d MB\n", len(globalData))
	fmt.Println("\n联合分析测试数据生成完成！")
	fmt.Println("运行: ./perfinspector testdata/profiles/cross/")
}
