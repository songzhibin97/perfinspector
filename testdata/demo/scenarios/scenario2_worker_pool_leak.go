//go:build ignore
// +build ignore

// 场景2：Worker Pool 泄漏 - Goroutine 和内存同时增长
// 模拟：创建 worker 但忘记关闭，每个 worker 持有一些内存
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

type Worker struct {
	id     int
	buffer []byte // 每个 worker 持有的内存
	done   chan struct{}
}

var workers []*Worker

func createWorker(id int, bufferSize int) *Worker {
	w := &Worker{
		id:     id,
		buffer: make([]byte, bufferSize),
		done:   make(chan struct{}),
	}

	// 启动 worker goroutine，但永远不会收到 done 信号
	go func() {
		<-w.done // 永远阻塞
	}()

	return w
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
	dir := "testdata/profiles/scenario2_worker_leak"
	os.MkdirAll(dir, 0755)

	fmt.Println("场景2：Worker Pool 泄漏（Goroutine 和内存同时增长）")
	fmt.Println("模拟：创建 worker 但忘记关闭，每个 worker 持有内存")

	for i := 1; i <= 3; i++ {
		// 创建一批 worker，每个持有 100KB 内存
		for j := 0; j < 20*i; j++ {
			w := createWorker(len(workers), 100*1024)
			workers = append(workers, w)
		}

		time.Sleep(100 * time.Millisecond)

		// 生成 heap profile
		heapFile := fmt.Sprintf("%s/heap_%d.pprof", dir, i)
		if err := generateHeapProfile(heapFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", heapFile, err)
		} else {
			fmt.Printf("✅ %s (Workers: %d)\n", heapFile, len(workers))
		}

		// 生成 goroutine profile
		goroutineFile := fmt.Sprintf("%s/goroutine_%d.pprof", dir, i)
		if err := generateGoroutineProfile(goroutineFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", goroutineFile, err)
		} else {
			fmt.Printf("✅ %s (Goroutines: %d)\n", goroutineFile, runtime.NumGoroutine())
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n预期结果：")
	fmt.Println("- 内存持续增长（每个 worker 100KB）")
	fmt.Println("- Goroutine 持续增长（每个 worker 一个 goroutine）")
	fmt.Println("- 应触发：Goroutine 泄漏导致内存增长（联合分析）")
}
