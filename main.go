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

var elog debug.Log
var config *ServiceConfig
var monitorInstance *monitor.Monitor

const configFileName = "service_config.json"

type myService struct{}

// 서비스 실행 로직
func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// args에는 ["is", "auto-started"]가 들어옴
	for _, arg := range args {
		elog.Info(1, fmt.Sprintf("인자: %s", arg))
	}

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// IO 모니터링 초기화
	monitorInstance = monitor.NewMonitor(10 * time.Second) // 모니터링 주기 설정
	elog.Info(1, "IO 모니터링이 초기화되었습니다.")

	// 기본 데이터베이스 경로 설정
	monitorInstance.SetDatabasePath(config.DatabasePath)

	// 모니터링 경로 설정
	for _, path := range config.MonitoringPath {
		monitorInstance.AddDevice(path)
	}

	// 파일 필터 설정
	monitorInstance.SetFileFilters([]string{".exe", ".dll"})

	// 모니터링 시작
	monitorInstance.Start()

	elog.Info(1, "IO 모니터링이 시작되었습니다.")

	// 서비스가 시작되면 Running 상태로 변경
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	elog.Info(1, fmt.Sprintf("서비스 '%s'가 시작되었습니다.", config.ServiceName))

	// 여기에 서비스의 메인 로직 구현
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			// 주기적으로 수행할 작업
			elog.Info(1, fmt.Sprintf("서비스 '%s'가 실행 중입니다.", config.ServiceName))

		case event := <-monitorInstance.EventChan():
			// 파일 이벤트 처리 - 이벤트 로그에만 기록
			elog.Info(1, fmt.Sprintf("파일 이벤트 발생: %s - %s", event.FileType, event.Path))
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				elog.Info(1, fmt.Sprintf("서비스 '%s'가 중지 요청을 받았습니다.", config.ServiceName))
				break loop
			default:
				elog.Error(1, fmt.Sprintf("예상치 못한 제어 요청 #%d", c))
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}

	// 정리 작업 수행
	if monitorInstance != nil {
		monitorInstance.Stop()
		elog.Info(1, "IO 모니터링이 중지되었습니다.")
	}

	// 서비스 종료
	changes <- svc.Status{State: svc.Stopped}
	elog.Info(1, fmt.Sprintf("서비스 '%s'가 종료되었습니다.", config.ServiceName))
	return
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			log.Fatalf("이벤트 로그를 열 수 없습니다: %v", err)
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("서비스 '%s'를 시작합니다.", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myService{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("서비스 실행 실패: %v", err))
		return
	}
	elog.Info(1, fmt.Sprintf("서비스 '%s'가 종료되었습니다.", name))
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
