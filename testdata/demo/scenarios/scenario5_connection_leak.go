//go:build ignore
// +build ignore

// 场景5：连接池泄漏 - 模拟数据库/HTTP 连接泄漏
// 模拟：获取连接但忘记释放
package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

type Connection struct {
	id     int
	conn   net.Conn
	buffer []byte
}

var connections []*Connection
var listener net.Listener

func init() {
	// 启动一个本地 TCP 服务器用于测试
	var err error
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	// 接受连接的 goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// 保持连接打开，模拟服务端
			go func(c net.Conn) {
				buf := make([]byte, 1024)
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
}

func getConnection() (*Connection, error) {
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		return nil, err
	}

	c := &Connection{
		id:     len(connections),
		conn:   conn,
		buffer: make([]byte, 64*1024), // 每个连接 64KB 缓冲区
	}
	return c, nil
}

// 忘记调用这个函数！
func releaseConnection(c *Connection) {
	c.conn.Close()
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
	dir := "testdata/profiles/scenario5_conn_leak"
	os.MkdirAll(dir, 0755)

	fmt.Println("场景5：连接池泄漏")
	fmt.Println("模拟：获取连接但忘记释放")

	for i := 1; i <= 3; i++ {
		// 获取连接但不释放
		for j := 0; j < 20*i; j++ {
			conn, err := getConnection()
			if err != nil {
				fmt.Printf("获取连接失败: %v\n", err)
				continue
			}
			connections = append(connections, conn)
			// 忘记调用 releaseConnection(conn)
		}

		time.Sleep(100 * time.Millisecond)

		// 生成 heap profile
		heapFile := fmt.Sprintf("%s/heap_%d.pprof", dir, i)
		if err := generateHeapProfile(heapFile); err != nil {
			fmt.Printf("生成 %s 失败: %v\n", heapFile, err)
		} else {
			fmt.Printf("✅ %s (连接数: %d)\n", heapFile, len(connections))
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

	listener.Close()

	fmt.Println("\n预期结果：")
	fmt.Println("- 内存持续增长（每个连接 64KB 缓冲区）")
	fmt.Println("- Goroutine 持续增长（每个连接一个读取 goroutine）")
	fmt.Println("- 应触发：Goroutine 泄漏导致内存增长")
	fmt.Println("- 热点应该是 net.Dial 和 buffer 分配")
}
