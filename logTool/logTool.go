// Package logTool 提供轻量结构化日志：date | level | thread-id | message。
// 配置从环境变量读取（由 getEnv 包加载 .env 后写入进程环境）：
//
//	LOG_LEVEL   DEBUG/INFO/WARN/ERROR，默认 INFO
//	LOG_FILE    日志文件路径，留空则只写控制台
//	LOG_FORMAT  text 或 json，默认 text
//	LOG_ROTATE  daily=每天新建并把旧文件改名为 <名>.<日期>.<后缀>，none=不轮转（默认）
//
// 输出格式：[包名] 日期 | 级别 | 协程号 | 文件:行号 | 消息
// 包名自动从调用方提取，无需配置。
//
// 用法：main 启动时调用一次 Init()，之后用 Debug/Info/Warn/Error 替代 fmt 打印。
package logTool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Level 日志级别，数值越大越严重。
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = [...]string{"DEBUG", "INFO", "WARN", "ERROR"}

var (
	mu          sync.Mutex
	minLevel    = INFO   // 低于此级别不输出
	logFormat   = "text" // text / json
	rotateDaily bool     // 是否按天轮转
	logPath     string   // 日志文件路径
	currentDate string   // 当前日志文件对应的日期 YYYY-MM-DD
	file        *os.File // nil 表示不写文件
)

// Init 从环境变量加载日志配置。可重复调用，会关闭上次打开的日志文件。
func Init() {
	mu.Lock()
	defer mu.Unlock()

	minLevel = parseLevel(os.Getenv("LOG_LEVEL"))
	logFormat = strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if logFormat != "json" {
		logFormat = "text"
	}
	rotateDaily = strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_ROTATE")), "daily")
	logPath = strings.TrimSpace(os.Getenv("LOG_FILE"))

	if file != nil {
		_ = file.Close()
		file = nil
	}
	if logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "日志模块: 创建目录失败:", err)
		return
	}
	currentDate = time.Now().Format("2006-01-02")
	if rotateDaily {
		// 启动时若已有旧日志文件（日期不是今天），先归档它
		archiveStaleLog()
	}
	file = openLogFile(logPath)
}

// openLogFile 以追加模式打开日志文件。
func openLogFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "日志模块: 打开日志文件失败:", err)
		return nil
	}
	return f
}

// archiveStaleLog 启动时检查已有日志文件：若其修改日期不是今天，改名为
// <文件名>.<日期>.<后缀>。用于程序跨天重启后归档上次遗留的日志。
func archiveStaleLog() {
	info, err := os.Stat(logPath)
	if err != nil || info.IsDir() {
		return
	}
	day := info.ModTime().Format("2006-01-02")
	if day == currentDate {
		return
	}
	if err := os.Rename(logPath, archivedName(day)); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "日志模块: 归档旧日志失败:", err)
	}
}

// archivedName 生成归档文件名：app.log → app.2026-11-24.log
func archivedName(day string) string {
	ext := filepath.Ext(logPath)             // .log
	base := strings.TrimSuffix(logPath, ext) // logs/app
	return base + "." + day + ext            // logs/app.2026-11-24.log
}

// rotateIfNeeded 每次写日志前检查：日期变了就关闭当前文件、改名归档、新建当天文件。
// 由 logf 持有 mu 调用。
func rotateIfNeeded() {
	if !rotateDaily || logPath == "" {
		return
	}
	today := time.Now().Format("2006-01-02")
	if today == currentDate {
		return
	}
	if file != nil {
		_ = file.Close()
	}
	if err := os.Rename(logPath, archivedName(currentDate)); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "日志模块: 跨天归档失败:", err)
	}
	file = openLogFile(logPath)
	currentDate = today
}

func parseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return DEBUG
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Debug 输出调试日志。
func Debug(format string, args ...any) { logf(DEBUG, format, args...) }

// Info 输出信息日志。
func Info(format string, args ...any) { logf(INFO, format, args...) }

// Warn 输出警告日志。
func Warn(format string, args ...any) { logf(WARN, format, args...) }

// Error 输出错误日志。
func Error(format string, args ...any) { logf(ERROR, format, args...) }

func logf(level Level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if level < minLevel {
		return
	}
	msg := fmt.Sprintf(format, args...)
	// skip=2: runtime.Caller → logf → Info/Debug/... → 业务代码调用处
	pc, callerFile, callerLine, _ := runtime.Caller(2)
	line := render(level, msg, callerFile, callerLine, pc)

	// DEBUG/INFO 走 stdout，WARN/ERROR 走 stderr，便于错误分流采集
	out := os.Stdout
	if level >= WARN {
		out = os.Stderr
	}
	_, _ = fmt.Fprintln(out, line)

	if file != nil {
		rotateIfNeeded()
		_, _ = fmt.Fprintln(file, line)
	}
}

func render(level Level, msg, callerFile string, callerLine int, pc uintptr) string {
	date := time.Now().Format("2006-01-02 15:04:05.000")
	lv := levelNames[level]
	tid := "g" + strconv.Itoa(goroutineID())
	caller := filepath.Base(callerFile) + ":" + strconv.Itoa(callerLine)

	// 包名自动从调用方函数名提取
	mod := packageName(pc)
	prefix := ""
	if mod != "" {
		prefix = "[" + mod + "] "
	}

	if logFormat == "json" {
		m := map[string]string{
			"date":      date,
			"level":     lv,
			"thread-id": tid,
			"caller":    caller,
			"message":   msg,
		}
		if mod != "" {
			m["module"] = mod
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	return fmt.Sprintf("%s%s | %-5s | %-4s | %-18s | %s", prefix, date, lv, tid, caller, msg)
}

// packageName 从调用点 PC 对应的函数名提取包名。
// 例：main.main → main；WorkChatBot/toolSDK.TestChat → toolSDK；
// main.(*Foo).Bar → main；闭包 main.main.func1 → main。
func packageName(pc uintptr) string {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	name := fn.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	return name
}

// goroutineID 从运行时栈里提取当前 goroutine id（非官方接口，仅用于日志关联）。
func goroutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := string(buf[:n])
	const prefix = "goroutine "
	if !strings.HasPrefix(s, prefix) {
		return 0
	}
	rest := s[len(prefix):]
	end := strings.IndexByte(rest, ' ')
	if end <= 0 {
		return 0
	}
	id, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0
	}
	return id
}
