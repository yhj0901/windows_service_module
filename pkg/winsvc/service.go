//go:build windows
// +build windows

package winsvc

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceConfig는 서비스 설정 정보를 담는 구조체
type ServiceConfig struct {
	ServiceName        string
	ServiceDescription string
	// 서비스 재시작 정책 설정
	RestartOnFailure   bool
	RestartDelay       int // 초 단위
	MaxRestartAttempts int
}

// ServiceManager는 Windows 서비스 관리를 위한 구조체
type ServiceManager struct {
	Config  *ServiceConfig
	Elog    debug.Log
	IsDebug bool
}

// NewServiceManager는 새로운 ServiceManager 인스턴스를 생성합니다
func NewServiceManager(config *ServiceConfig) *ServiceManager {
	return &ServiceManager{
		Config:  config,
		IsDebug: false,
	}
}

// Install은 서비스를 설치합니다
func (sm *ServiceManager) Install() error {
	exepath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로를 가져올 수 없습니다: %v", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(sm.Config.ServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("서비스 %s가 이미 존재합니다", sm.Config.ServiceName)
	}

	// 서비스 생성
	s, err = m.CreateService(sm.Config.ServiceName, exepath, mgr.Config{
		DisplayName:      sm.Config.ServiceName,
		Description:      sm.Config.ServiceDescription,
		StartType:        mgr.StartAutomatic,
		ServiceStartName: "", // LocalSystem 계정
	}, "is", "auto-started")
	if err != nil {
		return fmt.Errorf("서비스를 생성할 수 없습니다: %v", err)
	}
	defer s.Close()

	// 재시작 정책 설정
	if sm.Config.RestartOnFailure {
		// 재시작 설정은 서비스가 생성된 후 SetRecoveryActions 함수를 사용하여 설정
		// 일부 Windows 버전에서는 지원되지 않을 수 있음
		err = s.SetRecoveryActions([]mgr.RecoveryAction{
			{Type: mgr.ServiceRestart, Delay: time.Duration(sm.Config.RestartDelay) * time.Second},
			{Type: mgr.ServiceRestart, Delay: time.Duration(sm.Config.RestartDelay*2) * time.Second},
			{Type: mgr.ServiceRestart, Delay: time.Duration(sm.Config.RestartDelay*3) * time.Second},
		}, uint32(60)) // 60초 동안 오류가 없으면 카운터 리셋
		if err != nil {
			log.Printf("서비스 재시작 정책 설정 실패(무시됨): %v", err)
		}
	}

	// 이벤트 로그 생성
	err = eventlog.InstallAsEventCreate(sm.Config.ServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("이벤트 로그 설치 실패: %v", err)
	}

	fmt.Printf("서비스 '%s'가 설치되었습니다.\n", sm.Config.ServiceName)
	return nil
}

// Remove는 서비스를 제거합니다
func (sm *ServiceManager) Remove() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(sm.Config.ServiceName)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", sm.Config.ServiceName, err)
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
	err = eventlog.Remove(sm.Config.ServiceName)
	if err != nil {
		return fmt.Errorf("이벤트 로그를 제거할 수 없습니다: %v", err)
	}

	fmt.Printf("서비스 '%s'가 제거되었습니다.\n", sm.Config.ServiceName)
	return nil
}

// Start는 서비스를 시작합니다
func (sm *ServiceManager) Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(sm.Config.ServiceName)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", sm.Config.ServiceName, err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("서비스를 시작할 수 없습니다: %v", err)
	}

	fmt.Printf("서비스 '%s'가 시작되었습니다.\n", sm.Config.ServiceName)
	return nil
}

// Stop은 서비스를 중지합니다
func (sm *ServiceManager) Stop() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(sm.Config.ServiceName)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", sm.Config.ServiceName, err)
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

	fmt.Printf("서비스 '%s'가 중지되었습니다.\n", sm.Config.ServiceName)
	return nil
}

// Status는 서비스 상태를 확인합니다
func (sm *ServiceManager) Status() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("서비스 관리자에 연결할 수 없습니다: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(sm.Config.ServiceName)
	if err != nil {
		return fmt.Errorf("서비스 %s를 열 수 없습니다: %v", sm.Config.ServiceName, err)
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

	fmt.Printf("서비스 '%s'의 상태: %s\n", sm.Config.ServiceName, state)
	return nil
}

// Run은 서비스를 실행합니다
func (sm *ServiceManager) Run(handler svc.Handler) error {
	var err error

	if sm.IsDebug {
		sm.Elog = debug.New(sm.Config.ServiceName)
	} else {
		sm.Elog, err = eventlog.Open(sm.Config.ServiceName)
		if err != nil {
			return fmt.Errorf("이벤트 로그를 열 수 없습니다: %v", err)
		}
	}
	defer sm.Elog.Close()

	sm.Elog.Info(1, fmt.Sprintf("서비스 '%s'를 시작합니다.", sm.Config.ServiceName))

	run := svc.Run
	if sm.IsDebug {
		run = debug.Run
	}

	err = run(sm.Config.ServiceName, handler)
	if err != nil {
		sm.Elog.Error(1, fmt.Sprintf("서비스 실행 실패: %v", err))
		return err
	}

	sm.Elog.Info(1, fmt.Sprintf("서비스 '%s'가 종료되었습니다.", sm.Config.ServiceName))
	return nil
}

// IsWindowsService는 현재 프로세스가 Windows 서비스로 실행 중인지 확인합니다
func IsWindowsService() (bool, error) {
	return svc.IsWindowsService()
}
