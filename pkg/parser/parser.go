package parser

import (
	"log"
	"os"
	"time"

	"github.com/google/pprof/profile"
)

// GetProfileTime 从 pprof 元数据中提取时间戳
// 优先使用 StartTime，如果不存在则返回零值
func GetProfileTime(p *profile.Profile) time.Time {
	if p == nil {
		return time.Time{}
	}
	if p.TimeNanos > 0 {
		return time.Unix(0, p.TimeNanos).UTC()
	}
	return time.Time{}
}

// LoadProfile 加载并解析 pprof 文件
func LoadProfile(path string) (*profile.Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p, err := profile.Parse(f)
	if err != nil {
		return nil, err
	}

	timestamp := GetProfileTime(p)
	if timestamp.IsZero() {
		fileInfo, statErr := os.Stat(path)
		if statErr == nil {
			log.Printf("⏰ %s: 未找到元数据时间戳，回退到文件修改时间 (%s)",
				path, fileInfo.ModTime().Format(time.RFC3339))
		}
	} else {
		log.Printf("✅ %s: 使用pprof元数据时间戳 %s",
			path, timestamp.Format(time.RFC3339))
	}

	return p, nil
}
