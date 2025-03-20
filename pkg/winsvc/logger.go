//go:build windows
// +build windows

package winsvc

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc/debug"
)

// 로그 상수 정의
const (
	LogInfo    = "INFO"
	LogError   = "ERROR"
	LogWarning = "WARNING"
)

// Logger는 여러 로그 출력을 지원하는 로거입니다
type Logger struct {
	EventLog debug.Log   // Windows 이벤트 로그
	FileLog  *log.Logger // 파일 로거
	LogFile  *os.File    // 로그 파일 핸들
	IsDebug  bool        // 디버그 모드 여부
	LogPath  string      // 로그 파일 경로
}

// NewLogger는 새로운 Logger 인스턴스를 생성합니다
func NewLogger(logPath string, isDebug bool) *Logger {
	return &Logger{
		LogPath: logPath,
		IsDebug: isDebug,
	}
}

// InitializeFileLogger는 파일 로거를 초기화합니다
func (l *Logger) InitializeFileLogger() error {
	// 이미 열려있는 파일이 있다면 닫기
	if l.LogFile != nil {
		l.LogFile.Close()
	}

	// 로그 디렉토리 생성
	if err := os.MkdirAll(l.LogPath, 0755); err != nil {
		return fmt.Errorf("로그 디렉토리 생성 실패: %v", err)
	}

	var err error
	logFilePath := filepath.Join(l.LogPath, "service.log")
	l.LogFile, err = os.OpenFile(logFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("로그 파일 열기 실패: %v", err)
	}

	// 파일에만 로그 출력 (콘솔 출력 제거)
	l.FileLog = log.New(l.LogFile, "", log.LstdFlags)

	// 초기화 확인 로그
	l.FileLog.Printf("파일 로거가 초기화되었습니다. 경로: %s", logFilePath)
	return nil
}

// Close는 로거 리소스를 정리합니다
func (l *Logger) Close() {
	if l.LogFile != nil {
		l.LogFile.Close()
	}
}

// Log는 로그를 기록합니다
func (l *Logger) Log(level string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// 파일 로그
	if l.FileLog != nil {
		l.FileLog.Printf("[%s] %s", level, message)
	}

	// 이벤트 로그
	if l.EventLog != nil {
		switch level {
		case LogError:
			l.EventLog.Error(1, message)
		case LogWarning:
			l.EventLog.Warning(1, message)
		default:
			l.EventLog.Info(1, message)
		}
	}

	// 콘솔 출력 (디버그 모드일 때만)
	if l.IsDebug {
		log.Printf("[%s] %s", level, message)
	}
}
