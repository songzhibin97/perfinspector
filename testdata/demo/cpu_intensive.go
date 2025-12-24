//go:build ignore
// +build ignore

package main

import (
	"os"
	"runtime/pprof"
	"time"
)

// CPU 密集型任务
func cpuIntensive() {
	sum := 0
	for i := 0; i < 100000000; i++ {
		sum += i * i
	}
}

func main() {
	// 创建 CPU profile
	f, err := os.Create("testdata/profiles/cpu1.pprof")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	pprof.StartCPUProfile(f)

	// 执行 CPU 密集型任务
	for i := 0; i < 3; i++ {
		cpuIntensive()
	}

	time.Sleep(100 * time.Millisecond)
	pprof.StopCPUProfile()
}
