//go:build ignore
// +build ignore

// 场景3：Channel 泄漏 - 发送者阻塞导致 Goroutine 泄漏
// 模拟：创建 channel 但没有接收者，发送者永远阻塞
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var leakedChannels []chan int

func leakySender(ch chan int, data int) {
	// 这个 goroutine 会永远阻塞，因为没有接收者
	ch <- data
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
	dir := "testdata/profiles/scenario3_channel_leak"
	os.MkdirAll(dir, 0755)

	fmt.Println("场景3：Channel 泄漏（发送者阻塞）")
	fmt.Println("模拟：创建无缓冲 channel 但没有接收者")

	for i := 1; i <= 3; i++ {
		// 创建一批会泄漏的 goroutine
		for j := 0; j < 30*i; j++ {
			ch := make(chan int) // 无缓冲 channel
			leakedChannels = append(leakedChannels, ch)
			go leakySender(ch, j) // 发送者会永远阻塞
		}

		time.Sleep(100 * time.Millisecond)

		// 生成 heap profile
		heapFile := fmt.Sprintf("%s/heap_%d.pprof", dir, i)
		if err := generateHeapProfile(heapFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", heapFile, err)
		} else {
			fmt.Printf("✅ %s\n", heapFile)
		}

		// 生成 goroutine profile
		goroutineFile := fmt.Sprintf("%s/goroutine_%d.pprof", dir, i)
		if err := generateGoroutineProfile(goroutineFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", goroutineFile, err)
		} else {
			fmt.Printf("✅ %s (阻塞的 Goroutines: %d)\n", goroutineFile, len(leakedChannels))
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n预期结果：")
	fmt.Println("- Goroutine 持续增长（发送者阻塞）")
	fmt.Println("- 内存可能轻微增长（goroutine 栈）")
	fmt.Println("- 应触发：Goroutine 泄漏")
	fmt.Println("- 阻塞点应该是 runtime.chansend")
}
