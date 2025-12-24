//go:build ignore
// +build ignore

package main

import (
	"os"
	"runtime"
	"runtime/pprof"
)

var data [][]byte

// 内存分配任务
func allocateMemory() {
	for i := 0; i < 100; i++ {
		data = append(data, make([]byte, 1024*1024)) // 1MB
	}
}

func main() {
	// 分配内存
	allocateMemory()

	// 强制 GC
	runtime.GC()

	// 创建 heap profile
	f, err := os.Create("testdata/profiles/heap1.pprof")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	pprof.WriteHeapProfile(f)
}
