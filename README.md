# windows_service_module

Go 언어로 작성한 윈도우 서비스 모듈

## 기능

- Windows 서비스 등록/설치
- Windows 서비스 제거
- Windows 서비스 시작 및 중지
- 서비스 상태 확인
- 서비스 충돌 시 자동 재시작 기능
- 설정 파일을 통한 서비스 설정 관리

## 요구 사항

- Go 1.13 이상
- Windows 운영체제

## 빌드 방법

```bash
go build -o windows_service.exe
```

## 사용 방법

### 서비스 설치

```bash
windows_service.exe install
```

### 서비스 시작

```bash
windows_service.exe start
```

### 서비스 상태 확인

```bash
windows_service.exe status
```

### 서비스 중지

```bash
windows_service.exe stop
```

### 서비스 제거

```bash
windows_service.exe remove
```

### 디버그 모드로 실행

```bash
windows_service.exe debug
```

## 설정 파일

서비스는 실행 파일과 같은 디렉토리에 있는 `service_config.json` 파일을 통해 설정할 수 있습니다.

기본 설정 파일 형식:

```json
{
    "service_name": "MyWindowsService",
    "service_description": "My Windows Service Module",
    "restart_on_failure": true,
    "restart_delay": 5,
    "max_restart_attempts": 3,
    "log_path": "./logs",
    "custom_data_path": "./data"
}
```

## 주의 사항

- 이 모듈은 Windows 운영체제에서만 작동합니다.
- 서비스 설치 및 관리는 관리자 권한이 필요합니다.
