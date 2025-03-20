# Windows 서비스 모듈

윈도우 서비스를 쉽게 관리하고 배포할 수 있는 Go 언어 모듈입니다.

## 주요 기능

* 윈도우 서비스 등록, 삭제, 시작, 중지, 상태 확인
* 서비스 실패 시 자동 재시작 기능
* 파일 시스템 모니터링 (특정 확장자 파일 생성/수정/삭제 감지)
* 설정 파일을 통한 서비스 구성 관리
* 로그 기록 (파일 및 Windows 이벤트 로그)

## 프로젝트 구조

```
windows_service_module/
├── main.go              # 메인 애플리케이션 엔트리포인트
├── config.go            # 설정 파일 관리
├── go.mod               # Go 모듈 정의
├── service_config.json  # 서비스 설정 파일
├── pkg/                 # 패키지 디렉토리
│   └── winsvc/          # Windows 서비스 관리 패키지
│       ├── service.go   # 서비스 관리 기능
│       └── logger.go    # 로깅 기능
```

## 설치 및 사용법

### 1. 요구사항

* Windows 운영체제 (Windows 10/11 권장)
* Go 1.20 이상

### 2. 설치

```bash
# 저장소 클론
git clone https://github.com/yhj0901/windows_service_module.git
cd windows_service_module

# 의존성 설치
go mod tidy

# 빌드
go build -o windows_service.exe

# 모니터링 패키지 설치 (파일 모니터링 기능 사용 시)
go get github.com/yhj0901/windowsIOMonitoring
```

### 3. 서비스 설정

`service_config.json` 파일을 편집하여 서비스 설정을 구성할 수 있습니다:

```json
{
    "service_name": "hj-service",
    "service_description": "hj-service module",
    "restart_on_failure": true,
    "restart_delay": 5,
    "max_restart_attempts": 3,
    "log_path": ".\\logs",
    "database_path": ".\\db.sqlite",
    "monitoring_path": ["C:\\"],
    "custom_data_path": ".\\data"
}
```

### 4. 서비스 관리

```bash
# 서비스 설치
windows_service.exe install

# 서비스 시작
windows_service.exe start

# 서비스 상태 확인
windows_service.exe status

# 서비스 중지
windows_service.exe stop

# 서비스 제거
windows_service.exe remove

# 콘솔에서 디버그 모드로 실행
windows_service.exe debug
```

## 패키지 활용

프로젝트에서 직접 서비스 관리 패키지를 사용할 수 있습니다:

```go
import "windows_service_module/pkg/winsvc"

// 서비스 설정
config := &winsvc.ServiceConfig{
    ServiceName:        "my-service",
    ServiceDescription: "My Windows Service",
    RestartOnFailure:   true,
    RestartDelay:       5,
    MaxRestartAttempts: 3,
}

// 서비스 관리자 생성
manager := winsvc.NewServiceManager(config)

// 서비스 설치
if err := manager.Install(); err != nil {
    log.Fatalf("서비스 설치 실패: %v", err)
}

// 로거 생성
logger := winsvc.NewLogger("./logs", false)
logger.InitializeFileLogger()
defer logger.Close()

// 로그 기록
logger.Log(winsvc.LogInfo, "서비스 시작")
```

## 라이센스

MIT License

## 기여 방법

이슈 및 Pull Request 환영합니다.
