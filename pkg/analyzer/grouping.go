package analyzer

import (
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
	"github.com/songzhibin97/perfinspector/pkg/parser"
)

// ProfileFile 表示单个 profile 文件的信息
type ProfileFile struct {
	Path    string
	Time    time.Time
	Size    int64
	Profile *profile.Profile
	Metrics *ProfileMetrics // 性能指标
}

// ProfileGroup 表示按类型分组的 profile 集合
type ProfileGroup struct {
	Type  string
	Files []ProfileFile
}

// GroupProfiles 将 profile 文件按类型分组
func GroupProfiles(paths []string) ([]ProfileGroup, error) {
	groups := make(map[string][]ProfileFile)

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			log.Printf("❌ 文件不存在或无效: %s, 错误: %v", path, err)
			continue
		}

		p, err := parser.LoadProfile(path)
		if err != nil {
			log.Printf("⚠️ 跳过文件: %s, 错误: %v", path, err)
			continue
		}

		profileType := detectProfileType(p)
		if profileType == "" {
			profileType = "unknown"
		}

		timestamp := parser.GetProfileTime(p)
		if timestamp.IsZero() {
			timestamp = fileInfo.ModTime()
		}

		groups[profileType] = append(groups[profileType], ProfileFile{
			Path:    path,
			Time:    timestamp,
			Size:    fileInfo.Size(),
			Profile: p,
			Metrics: ExtractMetrics(p, profileType),
		})
	}

	var result []ProfileGroup
	for groupType, files := range groups {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Time.Before(files[j].Time)
		})
		result = append(result, ProfileGroup{
			Type:  groupType,
			Files: files,
		})
	}

	// 按类型名称排序，保证输出顺序一致
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})

	return result, nil
}

// detectProfileType 检测 profile 的类型
func detectProfileType(p *profile.Profile) string {
	if p == nil {
		return "unknown"
	}

	// 检查 SampleType 来判断类型
	if len(p.SampleType) > 0 {
		for _, st := range p.SampleType {
			typeLower := strings.ToLower(st.Type)
			unitLower := strings.ToLower(st.Unit)

			// CPU profile
			if typeLower == "cpu" || typeLower == "samples" {
				if unitLower == "nanoseconds" || unitLower == "count" {
					return "cpu"
				}
			}

			// Heap/Memory profile
			if typeLower == "alloc_objects" || typeLower == "alloc_space" ||
				typeLower == "inuse_objects" || typeLower == "inuse_space" {
				return "heap"
			}

			// Goroutine profile
			if typeLower == "goroutine" || unitLower == "goroutine" {
				return "goroutine"
			}

			// Block profile
			if typeLower == "contentions" || typeLower == "delay" {
				return "block"
			}

			// Mutex profile
			if typeLower == "contentions" && unitLower == "count" {
				return "mutex"
			}
		}
	}

	// 通过 Duration 判断 CPU profile
	if p.DurationNanos > 0 {
		return "cpu"
	}

	return "unknown"
}
