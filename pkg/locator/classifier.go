package locator

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Classifier 代码分类器
type Classifier struct {
	moduleName         string
	thirdPartyPrefixes []string
	stdlibPackages     map[string]bool // 预加载的标准库包列表
}

// NewClassifier 创建分类器
func NewClassifier(config LocatorConfig) *Classifier {
	c := &Classifier{
		moduleName:         config.ModuleName,
		thirdPartyPrefixes: config.ThirdPartyPrefixes,
		stdlibPackages:     make(map[string]bool),
	}

	// 初始化标准库包列表
	for _, pkg := range goStdlibPackages {
		c.stdlibPackages[pkg] = true
	}

	return c
}

// Classify 对包名进行分类
func (c *Classifier) Classify(packageName string) CodeCategory {
	if packageName == "" {
		return CategoryUnknown
	}

	// 1. 检查是否是 runtime 包
	if c.isRuntimePackage(packageName) {
		return CategoryRuntime
	}

	// 2. 检查是否是标准库包
	if c.isStdlibPackage(packageName) {
		return CategoryStdlib
	}

	// 3. 检查是否是业务代码（用户模块）
	if c.isBusinessPackage(packageName) {
		return CategoryBusiness
	}

	// 4. 检查是否是第三方包
	if c.isThirdPartyPackage(packageName) {
		return CategoryThirdParty
	}

	return CategoryUnknown
}

// isRuntimePackage 检查是否是 Go 运行时包
func (c *Classifier) isRuntimePackage(packageName string) bool {
	return packageName == "runtime" || strings.HasPrefix(packageName, "runtime/")
}

// isStdlibPackage 检查是否是 Go 标准库包
func (c *Classifier) isStdlibPackage(packageName string) bool {
	// 检查完整包名
	if c.stdlibPackages[packageName] {
		return true
	}

	// 检查是否是标准库子包 (如 net/http/httptest)
	// 通过检查顶级包是否在标准库列表中
	topLevel := packageName
	if idx := strings.Index(packageName, "/"); idx > 0 {
		topLevel = packageName[:idx]
	}

	// 如果顶级包在标准库中，且不包含 "." (排除域名)
	if c.stdlibPackages[topLevel] && !strings.Contains(topLevel, ".") {
		return true
	}

	// golang.org/x/* 被视为扩展标准库
	if strings.HasPrefix(packageName, "golang.org/x/") {
		return true
	}

	return false
}

// isBusinessPackage 检查是否是业务代码包
func (c *Classifier) isBusinessPackage(packageName string) bool {
	// 新增: main 包始终是业务代码
	if packageName == "main" || strings.HasPrefix(packageName, "main.") {
		return true
	}

	// 新增: 不包含 "/" 且不是标准库/运行时的包视为业务代码
	// 这处理了像 "mypackage.MyFunc" 这样的本地包函数
	if !strings.Contains(packageName, "/") &&
		!c.isRuntimePackage(packageName) &&
		!c.isStdlibPackage(packageName) {
		return true
	}

	// 原有逻辑: 检查模块名
	if c.moduleName != "" {
		return packageName == c.moduleName || strings.HasPrefix(packageName, c.moduleName+"/")
	}

	return false
}

// isThirdPartyPackage 检查是否是第三方包
func (c *Classifier) isThirdPartyPackage(packageName string) bool {
	// 检查用户配置的第三方前缀
	for _, prefix := range c.thirdPartyPrefixes {
		if strings.HasPrefix(packageName, prefix) {
			return true
		}
	}

	// 常见的第三方包域名
	thirdPartyDomains := []string{
		"github.com/",
		"gitlab.com/",
		"bitbucket.org/",
		"gopkg.in/",
		"go.uber.org/",
		"google.golang.org/",
		"cloud.google.com/",
		"k8s.io/",
		"sigs.k8s.io/",
	}

	for _, domain := range thirdPartyDomains {
		if strings.HasPrefix(packageName, domain) {
			// 排除用户自己的模块
			if c.moduleName != "" && strings.HasPrefix(packageName, c.moduleName) {
				return false
			}
			return true
		}
	}

	return false
}

// DetectModuleName 从 go.mod 检测模块名
func DetectModuleName(workDir string) (string, error) {
	goModPath := filepath.Join(workDir, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			// 提取模块名
			moduleName := strings.TrimPrefix(line, "module ")
			moduleName = strings.TrimSpace(moduleName)
			return moduleName, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", os.ErrNotExist
}

// goStdlibPackages Go 标准库包列表
var goStdlibPackages = []string{
	// 基础包
	"archive",
	"archive/tar",
	"archive/zip",
	"bufio",
	"builtin",
	"bytes",
	"compress",
	"compress/bzip2",
	"compress/flate",
	"compress/gzip",
	"compress/lzw",
	"compress/zlib",
	"container",
	"container/heap",
	"container/list",
	"container/ring",
	"context",
	"crypto",
	"crypto/aes",
	"crypto/cipher",
	"crypto/des",
	"crypto/dsa",
	"crypto/ecdh",
	"crypto/ecdsa",
	"crypto/ed25519",
	"crypto/elliptic",
	"crypto/hmac",
	"crypto/md5",
	"crypto/rand",
	"crypto/rc4",
	"crypto/rsa",
	"crypto/sha1",
	"crypto/sha256",
	"crypto/sha512",
	"crypto/subtle",
	"crypto/tls",
	"crypto/x509",
	"crypto/x509/pkix",
	"database",
	"database/sql",
	"database/sql/driver",
	"debug",
	"debug/buildinfo",
	"debug/dwarf",
	"debug/elf",
	"debug/gosym",
	"debug/macho",
	"debug/pe",
	"debug/plan9obj",
	"embed",
	"encoding",
	"encoding/ascii85",
	"encoding/asn1",
	"encoding/base32",
	"encoding/base64",
	"encoding/binary",
	"encoding/csv",
	"encoding/gob",
	"encoding/hex",
	"encoding/json",
	"encoding/pem",
	"encoding/xml",
	"errors",
	"expvar",
	"flag",
	"fmt",
	"go",
	"go/ast",
	"go/build",
	"go/build/constraint",
	"go/constant",
	"go/doc",
	"go/doc/comment",
	"go/format",
	"go/importer",
	"go/parser",
	"go/printer",
	"go/scanner",
	"go/token",
	"go/types",
	"hash",
	"hash/adler32",
	"hash/crc32",
	"hash/crc64",
	"hash/fnv",
	"hash/maphash",
	"html",
	"html/template",
	"image",
	"image/color",
	"image/color/palette",
	"image/draw",
	"image/gif",
	"image/jpeg",
	"image/png",
	"index",
	"index/suffixarray",
	"io",
	"io/fs",
	"io/ioutil",
	"log",
	"log/slog",
	"log/syslog",
	"maps",
	"math",
	"math/big",
	"math/bits",
	"math/cmplx",
	"math/rand",
	"mime",
	"mime/multipart",
	"mime/quotedprintable",
	"net",
	"net/http",
	"net/http/cgi",
	"net/http/cookiejar",
	"net/http/fcgi",
	"net/http/httptest",
	"net/http/httptrace",
	"net/http/httputil",
	"net/http/pprof",
	"net/mail",
	"net/netip",
	"net/rpc",
	"net/rpc/jsonrpc",
	"net/smtp",
	"net/textproto",
	"net/url",
	"os",
	"os/exec",
	"os/signal",
	"os/user",
	"path",
	"path/filepath",
	"plugin",
	"reflect",
	"regexp",
	"regexp/syntax",
	"slices",
	"sort",
	"strconv",
	"strings",
	"sync",
	"sync/atomic",
	"syscall",
	"testing",
	"testing/fstest",
	"testing/iotest",
	"testing/quick",
	"text",
	"text/scanner",
	"text/tabwriter",
	"text/template",
	"text/template/parse",
	"time",
	"time/tzdata",
	"unicode",
	"unicode/utf16",
	"unicode/utf8",
	"unsafe",
	// internal 包 (通常不直接使用，但可能出现在调用栈中)
	"internal",
}
