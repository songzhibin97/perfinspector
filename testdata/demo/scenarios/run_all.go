//go:build ignore
// +build ignore

// 运行所有场景并生成报告
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	scenarios := []struct {
		name   string
		file   string
		outDir string
	}{
		{"场景1: 缓存泄漏", "scenario1_cache_leak.go", "scenario1_cache_leak"},
		{"场景2: Worker Pool 泄漏", "scenario2_worker_pool_leak.go", "scenario2_worker_leak"},
		{"场景3: Channel 泄漏", "scenario3_channel_leak.go", "scenario3_channel_leak"},
		{"场景4: CPU 热点", "scenario4_cpu_hotspot.go", "scenario4_cpu_hotspot"},
		{"场景5: 连接池泄漏", "scenario5_connection_leak.go", "scenario5_conn_leak"},
	}

	fmt.Println("=" + string(make([]byte, 60)))
	fmt.Println("运行所有问题场景并生成 pprof 文件")
	fmt.Println("=" + string(make([]byte, 60)))

	baseDir := "testdata/demo/scenarios"

	for i, s := range scenarios {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(scenarios), s.name)
		fmt.Println("-" + string(make([]byte, 50)))

		cmd := exec.Command("go", "run", filepath.Join(baseDir, s.file))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("运行 %s 失败: %v\n", s.file, err)
			continue
		}
	}

	fmt.Println("\n" + "=" + string(make([]byte, 60)))
	fmt.Println("所有场景运行完成！")
	fmt.Println("=" + string(make([]byte, 60)))

	fmt.Println("\n现在可以运行 PerfInspector 分析各场景：")
	for _, s := range scenarios {
		fmt.Printf("  ./perfinspector testdata/profiles/%s/\n", s.outDir)
	}
}
