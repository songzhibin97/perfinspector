//go:build ignore
// +build ignore

// 场景1：缓存泄漏 - 内存持续增长但 Goroutine 稳定
// 模拟：缓存没有过期策略，数据只进不出
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// 模拟一个没有过期策略的缓存
var cache = make(map[string][]byte)

func addToCache(key string, size int) {
	cache[key] = make([]byte, size)
}

func generateHeapProfile(filename string) error {
	runtime.GC()
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.WriteHeapProfile(f)
}

func generateGoroutineProfile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return pprof.Lookup("goroutine").WriteTo(f, 0)
}

func main() {
	dir := "testdata/profiles/scenario1_cache_leak"
	os.MkdirAll(dir, 0755)

	fmt.Println("场景1：缓存泄漏（内存增长，Goroutine 稳定）")
	fmt.Println("模拟：缓存没有过期策略，数据只进不出")

	for i := 1; i <= 3; i++ {
		// 往缓存里塞数据，不删除
		for j := 0; j < 1000; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			addToCache(key, 1024*i) // 每轮增加更多数据
		}

		time.Sleep(100 * time.Millisecond)

		// 生成 heap profile
		heapFile := fmt.Sprintf("%s/heap_%d.pprof", dir, i)
		if err := generateHeapProfile(heapFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", heapFile, err)
		} else {
			fmt.Printf("✅ %s (缓存条目: %d, 内存: ~%d KB)\n",
				heapFile, len(cache), len(cache)*i)
		}

		// 生成 goroutine profile（应该稳定）
		goroutineFile := fmt.Sprintf("%s/goroutine_%d.pprof", dir, i)
		if err := generateGoroutineProfile(goroutineFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", goroutineFile, err)
		} else {
			fmt.Printf("✅ %s (Goroutines: %d)\n", goroutineFile, runtime.NumGoroutine())
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n预期结果：")
	fmt.Println("- 内存持续增长（heap 趋势 increasing）")
	fmt.Println("- Goroutine 数量稳定")
	fmt.Println("- 应触发：独立内存泄漏（非 Goroutine 相关）")
}
