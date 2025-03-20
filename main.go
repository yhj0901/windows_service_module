//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"windows_service_module/pkg/winsvc"

	"github.com/yhj0901/windowsIOMonitoring/pkg/monitor"
	"golang.org/x/sys/windows/svc"
)

var (
	config          *ServiceConfig
	monitorInstance *monitor.Monitor
	serviceManager  *winsvc.ServiceManager
	logger          *winsvc.Logger
)

const configFileName = "service_config.json"

type myService struct{}

// 서비스 실행 로직
func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	defer logger.Close()

	for _, arg := range args {
		logger.Log(winsvc.LogInfo, "인자: %s", arg)
	}

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// 디렉토리 초기화 추가
	if err := initializeDirectories(); err != nil {
		logger.Log(winsvc.LogError, "디렉토리 초기화 실패: %v", err)
		return
	}

	// 파일 로거 초기화 추가
	if err := logger.InitializeFileLogger(); err != nil {
		logger.Log(winsvc.LogError, "로그 초기화 실패: %v", err)
		return
	}

	// IO 모니터링 초기화
	monitorInstance = monitor.NewMonitor(10 * time.Second)
	logger.Log(winsvc.LogInfo, "IO 모니터링이 초기화되었습니다.")

	// 기본 데이터베이스 경로 설정
	logger.Log(winsvc.LogInfo, "데이터베이스 경로 설정: %s", config.DatabasePath)
	monitorInstance.SetDatabasePath(config.DatabasePath)

	// 모니터링 경로 설정
	for _, path := range config.MonitoringPath {
		monitorInstance.AddDevice(path)
	}

	// 파일 필터 설정
	monitorInstance.SetFileFilters([]string{".exe", ".dll"})

	// 모니터링 시작
	if err := monitorInstance.Start(); err != nil {
		logger.Log(winsvc.LogError, "모니터링 시작 실패: %v", err)
		return
	}
	logger.Log(winsvc.LogInfo, "모니터링이 성공적으로 시작되었습니다")

	// 서비스가 시작되면 Running 상태로 변경
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	logger.Log(winsvc.LogInfo, "서비스 '%s'가 시작되었습니다.", config.ServiceName)

	// 여기에 서비스의 메인 로직 구현
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			// 주기적으로 수행할 작업
			logger.Log(winsvc.LogInfo, "서비스 '%s'가 실행 중입니다.", config.ServiceName)

		case event := <-monitorInstance.EventChan():
			// 파일 이벤트 처리 - 이벤트 로그와 파일 로그에 기록
			logger.Log(winsvc.LogInfo, "파일 이벤트 발생: %s - %s", event.FileType, event.Path)
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				logger.Log(winsvc.LogInfo, "서비스 '%s'가 중지 요청을 받았습니다.", config.ServiceName)
				break loop
			default:
				logger.Log(winsvc.LogError, "예상치 못한 제어 요청 #%d", c)
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}

	// 정리 작업 수행
	if monitorInstance != nil {
		monitorInstance.Stop()
		logger.Log(winsvc.LogInfo, "IO 모니터링이 중지되었습니다.")
	}

	// 서비스 종료
	changes <- svc.Status{State: svc.Stopped}
	logger.Log(winsvc.LogInfo, "서비스 '%s'가 종료되었습니다.", config.ServiceName)
	return
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

func main() {
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

	// 로깅 초기화
	logger = winsvc.NewLogger(config.LogPath, false)

	// 서비스 관리자 초기화
	svcConfig := &winsvc.ServiceConfig{
		ServiceName:        config.ServiceName,
		ServiceDescription: config.ServiceDescription,
		RestartOnFailure:   config.RestartOnFailure,
		RestartDelay:       config.RestartDelay,
		MaxRestartAttempts: config.MaxRestartAttempts,
	}
	serviceManager = winsvc.NewServiceManager(svcConfig)

	// 인자가 없으면 서비스로 실행
	isWindowsService, err := winsvc.IsWindowsService()
	if err != nil {
		log.Fatalf("윈도우 서비스 확인 실패: %v", err)
	}

	if isWindowsService {
		// 서비스로 실행
		serviceManager.IsDebug = false
		logger.EventLog = serviceManager.Elog
		if err := serviceManager.Run(&myService{}); err != nil {
			log.Fatalf("서비스 실행 실패: %v", err)
		}
		return
	}

	// 명령행 인자에 따라 다른 동작 수행
	if len(os.Args) < 2 {
		usage("명령이 지정되지 않았습니다")
	}

	cmd := os.Args[1]
	switch cmd {
	case "install":
		err = serviceManager.Install()
	case "remove":
		err = serviceManager.Remove()
	case "start":
		err = serviceManager.Start()
	case "stop":
		err = serviceManager.Stop()
	case "status":
		err = serviceManager.Status()
	case "debug":
		// 디버그 모드로 실행
		serviceManager.IsDebug = true
		logger.IsDebug = true
		logger.EventLog = serviceManager.Elog
		if err := serviceManager.Run(&myService{}); err != nil {
			log.Fatalf("서비스 실행 실패: %v", err)
		}
		return
	default:
		usage(fmt.Sprintf("알 수 없는 명령: %s", cmd))
	}

	if err != nil {
		log.Fatalf("명령 실행 중 오류 발생: %v", err)
	}
}
