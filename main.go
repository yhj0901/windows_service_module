//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/yhj0901/windowsIOMonitoring/pkg/monitor"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	elog            debug.Log
	config          *ServiceConfig
	monitorInstance *monitor.Monitor
	fileLogger      *log.Logger
	logFile         *os.File // 추가: 로그 파일 핸들러
	isDebug         bool
)

const configFileName = "service_config.json"

// 로그 상수 정의
const (
	LogInfo    = "INFO"
	LogError   = "ERROR"
	LogWarning = "WARNING"
)

type myService struct{}

// 서비스 실행 로직
func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	for _, arg := range args {
		logMessage(LogInfo, "인자: %s", arg)
	}

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// 디렉토리 초기화 추가
	if err := initializeDirectories(); err != nil {
		logMessage(LogError, "디렉토리 초기화 실패: %v", err)
		return
	}

	// 파일 로거 초기화 추가
	if err := initializeFileLogger(); err != nil {
		logMessage(LogError, "로그 초기화 실패: %v", err)
		return
	}

	// IO 모니터링 초기화
	monitorInstance = monitor.NewMonitor(10 * time.Second)
	logMessage(LogInfo, "IO 모니터링이 초기화되었습니다.")

	// 기본 데이터베이스 경로 설정
	logMessage(LogInfo, "데이터베이스 경로 설정: %s", config.DatabasePath)
	monitorInstance.SetDatabasePath(config.DatabasePath)

	// 모니터링 경로 설정
	for _, path := range config.MonitoringPath {
		monitorInstance.AddDevice(path)
	}

	// 파일 필터 설정
	monitorInstance.SetFileFilters([]string{".exe", ".dll"})

	// 모니터링 시작
	if err := monitorInstance.Start(); err != nil {
		logMessage(LogError, "모니터링 시작 실패: %v", err)
		return
	}
	logMessage(LogInfo, "모니터링이 성공적으로 시작되었습니다")

	// 서비스가 시작되면 Running 상태로 변경
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	logMessage(LogInfo, "서비스 '%s'가 시작되었습니다.", config.ServiceName)

	// 여기에 서비스의 메인 로직 구현
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			// 주기적으로 수행할 작업
			logMessage(LogInfo, "서비스 '%s'가 실행 중입니다.", config.ServiceName)

		case event := <-monitorInstance.EventChan():
			// 파일 이벤트 처리 - 이벤트 로그와 파일 로그에 기록
			logMessage(LogInfo, "파일 이벤트 발생: %s - %s", event.FileType, event.Path)
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				logMessage(LogInfo, "서비스 '%s'가 중지 요청을 받았습니다.", config.ServiceName)
				break loop
			default:
				logMessage(LogError, "예상치 못한 제어 요청 #%d", c)
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}

	// 정리 작업 수행
	if monitorInstance != nil {
		monitorInstance.Stop()
		logMessage(LogInfo, "IO 모니터링이 중지되었습니다.")
	}

	// 서비스 종료
	changes <- svc.Status{State: svc.Stopped}
	logMessage(LogInfo, "서비스 '%s'가 종료되었습니다.", config.ServiceName)
	return
}

func runService(name string, debugMode bool) {
	isDebug = debugMode
	var err error

	// 디렉토리 초기화
	if err := initializeDirectories(); err != nil {
		log.Fatalf("디렉토리 초기화 실패: %v", err)
		return
	}

	// 파일 로거 초기화
	if err := initializeFileLogger(); err != nil {
		log.Fatalf("로그 초기화 실패: %v", err)
		return
	}

	// 로그 테스트
	fileLogger.Printf("runService 함수 시작: %s", name)
	logMessage(LogInfo, "runService 함수 시작: %s", name) // logMessage 사용

	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			fileLogger.Printf("이벤트 로그를 열 수 없습니다: %v", err)
			log.Fatalf("이벤트 로그를 열 수 없습니다: %v", err)
			return
		}
	}
	defer elog.Close()

	fileLogger.Printf("서비스 '%s'를 시작합니다.", name)
	logMessage(LogInfo, "서비스 '%s'를 시작합니다.", name)

	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myService{})
	if err != nil {
		fileLogger.Printf("서비스 실행 실패: %v", err)
		logMessage(LogError, "서비스 실행 실패: %v", err)
		return
	}
	fileLogger.Printf("서비스 '%s'가 종료되었습니다.", name)
	logMessage(LogInfo, "서비스 '%s'가 종료되었습니다.", name)
}

// 서비스 설치
func installService(name, desc string) error {
	exepath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로를 가져올 수 없습니다: %v", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("서비스 %s가 이미 존재합니다", name)
	}

	// 서비스 생성
	s, err = m.CreateService(name, exepath, mgr.Config{
		DisplayName:      name,
		Description:      desc,
		StartType:        mgr.StartAutomatic,
		ServiceStartName: "", // LocalSystem 계정
	}, "is", "auto-started")
	if err != nil {
		return fmt.Errorf("서비스를 생성할 수 없습니다: %v", err)
	}
	defer s.Close()

	// 재시작 정책 설정
	if config.RestartOnFailure {
		// 재시작 설정은 서비스가 생성된 후 SetRecoveryActions 함수를 사용하여 설정
		// 일부 Windows 버전에서는 지원되지 않을 수 있음
		err = s.SetRecoveryActions([]mgr.RecoveryAction{
			{Type: mgr.ServiceRestart, Delay: time.Duration(config.RestartDelay) * time.Second},
			{Type: mgr.ServiceRestart, Delay: time.Duration(config.RestartDelay*2) * time.Second},
			{Type: mgr.ServiceRestart, Delay: time.Duration(config.RestartDelay*3) * time.Second},
		}, uint32(60)) // 60초 동안 오류가 없으면 카운터 리셋
		if err != nil {
			log.Printf("서비스 재시작 정책 설정 실패(무시됨): %v", err)
		}
	}

	// 이벤트 로그 생성
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("이벤트 로그 설치 실패: %v", err)
	}

	fmt.Printf("서비스 '%s'가 설치되었습니다.\n", name)
	return nil
}

// 서비스 제거
func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", name, err)
	}
	defer s.Close()

	// 서비스 중지
	status, err := s.Control(svc.Stop)
	if err != nil {
		// 중지 실패는 무시하고 계속 진행
		fmt.Printf("서비스를 중지하는 중 오류 발생 (무시됨): %v\n", err)
	} else {
		// 서비스가 완전히 중지될 때까지 대기
		for status.State != svc.Stopped {
			time.Sleep(500 * time.Millisecond)
			status, err = s.Query()
			if err != nil {
				break
			}
		}
	}

	// 서비스 삭제
	err = s.Delete()
	if err != nil {
		return fmt.Errorf("서비스를 삭제할 수 없습니다: %v", err)
	}

	// 이벤트 로그 제거
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("이벤트 로그를 제거할 수 없습니다: %v", err)
	}

	fmt.Printf("서비스 '%s'가 제거되었습니다.\n", name)
	return nil
}

// 서비스 시작
func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", name, err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("서비스를 시작할 수 없습니다: %v", err)
	}

	fmt.Printf("서비스 '%s'가 시작되었습니다.\n", name)
	return nil
}

// 서비스 중지
func stopService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", name, err)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("서비스를 중지할 수 없습니다: %v", err)
	}

	// 서비스가 완전히 중지될 때까지 대기
	for status.State != svc.Stopped {
		time.Sleep(500 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("서비스 상태를 확인할 수 없습니다: %v", err)
		}
	}

	fmt.Printf("서비스 '%s'가 중지되었습니다.\n", name)
	return nil
}

// 서비스 상태 확인
func statusService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", name, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("서비스 상태를 확인할 수 없습니다: %v", err)
	}

	var state string
	switch status.State {
	case svc.Running:
		state = "실행 중"
	case svc.Stopped:
		state = "중지됨"
	case svc.StartPending:
		state = "시작 중"
	case svc.StopPending:
		state = "중지 중"
	case svc.PausePending:
		state = "일시 중지 중"
	case svc.Paused:
		state = "일시 중지됨"
	case svc.ContinuePending:
		state = "계속 중"
	default:
		state = fmt.Sprintf("알 수 없음 (%d)", status.State)
	}

	fmt.Printf("서비스 '%s'의 상태: %s\n", name, state)
	return nil
}

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"사용법:\n"+
			"  %s install    - 서비스 설치\n"+
			"  %s remove     - 서비스 제거\n"+
			"  %s start      - 서비스 시작\n"+
			"  %s stop       - 서비스 중지\n"+
			"  %s status     - 서비스 상태 확인\n"+
			"  %s debug      - 콘솔에서 서비스 실행\n",
		errmsg, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	os.Exit(1)
}

func initializeDirectories() error {
	// 절대 경로로 변환
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return fmt.Errorf("실행 파일 경로를 가져올 수 없습니다: %v", err)
	}

	// 설정의 상대 경로가 이미 절대 경로인지 확인
	var logPath, dbPath, dataPath string

	if filepath.IsAbs(config.LogPath) {
		logPath = config.LogPath
	} else {
		logPath = filepath.Join(execDir, config.LogPath)
	}

	if filepath.IsAbs(config.DatabasePath) {
		dbPath = config.DatabasePath
	} else {
		dbPath = filepath.Join(execDir, config.DatabasePath)
	}

	if filepath.IsAbs(config.CustomDataPath) {
		dataPath = config.CustomDataPath
	} else {
		dataPath = filepath.Join(execDir, config.CustomDataPath)
	}

	// 설정 업데이트
	config.LogPath = filepath.Clean(logPath)
	config.DatabasePath = filepath.Clean(dbPath)
	config.CustomDataPath = filepath.Clean(dataPath)

	// 디버그 로그
	log.Printf("로그 경로: %s", config.LogPath)
	log.Printf("DB 경로: %s", config.DatabasePath)
	log.Printf("데이터 경로: %s", config.CustomDataPath)

	dirs := []string{
		config.LogPath,
		filepath.Dir(config.DatabasePath),
		config.CustomDataPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("디렉토리 생성 실패 %s: %v", dir, err)
		}
		log.Printf("디렉토리 생성됨: %s", dir)
	}

	return nil
}

func initializeFileLogger() error {
	// 이미 열려있는 파일이 있다면 닫기
	if logFile != nil {
		logFile.Close()
	}

	// 로그 디렉토리 생성
	if err := os.MkdirAll(config.LogPath, 0755); err != nil {
		return fmt.Errorf("로그 디렉토리 생성 실패: %v", err)
	}

	var err error
	logPath := filepath.Join(config.LogPath, "service.log")
	logFile, err = os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("로그 파일 열기 실패: %v", err)
	}

	// 파일에만 로그 출력 (콘솔 출력 제거)
	fileLogger = log.New(logFile, "", log.LstdFlags)

	// 초기화 확인 로그
	fileLogger.Printf("파일 로거가 초기화되었습니다. 경로: %s", logPath)
	return nil
}

// 통합 로그 함수
func logMessage(level string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// 파일 로그
	if fileLogger != nil {
		fileLogger.Printf("[%s] %s", level, message)
	}

	// 이벤트 로그
	if elog != nil {
		switch level {
		case LogError:
			elog.Error(1, message)
		case LogWarning:
			elog.Warning(1, message)
		default:
			elog.Info(1, message)
		}
	}

	// 콘솔 출력 (디버그 모드일 때만)
	if isDebug {
		log.Printf("[%s] %s", level, message)
	}
}

func main() {
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	// 설정 로드
	// 실행파일이 있는 경로만 추출하기
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("실행 파일 경로를 가져올 수 없습니다: %v", err)
	}

	configPath := filepath.Join(execDir, configFileName)
	EnsureDefaultConfig(configPath)

	config, err = LoadConfig(configPath)
	if err != nil {
		log.Fatalf("설정을 로드할 수 없습니다: %v", err)
	}

	// 인자가 없으면 서비스로 실행
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("윈도우 서비스 확인 실패: %v", err)
	}
	if isWindowsService {
		runService(config.ServiceName, false)
		return
	}

	// 명령행 인자에 따라 다른 동작 수행
	if len(os.Args) < 2 {
		usage("명령이 지정되지 않았습니다")
	}

	cmd := os.Args[1]
	switch cmd {
	case "install":
		err = installService(config.ServiceName, config.ServiceDescription)
	case "remove":
		err = removeService(config.ServiceName)
	case "start":
		err = startService(config.ServiceName)
	case "stop":
		err = stopService(config.ServiceName)
	case "status":
		err = statusService(config.ServiceName)
	case "debug":
		runService(config.ServiceName, true)
		return
	default:
		usage(fmt.Sprintf("알 수 없는 명령: %s", cmd))
	}

	if err != nil {
		log.Fatalf("명령 실행 중 오류 발생: %v", err)
	}
}
