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

## 빌드 요구사항

### SQLite3 지원을 위한 CGO 설정

이 서비스는 SQLite3를 사용하기 때문에 CGO가 활성화된 상태로 빌드해야 합니다.

#### 1. C 컴파일러 설치 (MSYS2 + MinGW-w64)

1. [MSYS2 설치](https://www.msys2.org/)

2. MSYS2 터미널에서 MinGW-w64 설치:
```bash
pacman -S mingw-w64-x86_64-gcc
```

3. 시스템 환경 변수 PATH에 다음 경로들을 **순서대로** 추가:
```
C:\msys64\mingw64\bin
```

4. 설치 확인:
```bash
gcc --version
```

#### 2. 빌드 환경 설정

PowerShell 또는 명령 프롬프트에서:

```bash
# CGO 활성화
set CGO_ENABLED=1

# 빌드
go build -tags "windows" -ldflags "-H=windowsgui" -buildmode=exe
```

또는 한 줄로 실행:

```bash
set CGO_ENABLED=1 && go build -tags "windows" -ldflags "-H=windowsgui" -buildmode=exe
```

### 문제 해결

만약 여전히 "CGO_ENABLED=0" 에러가 발생한다면:

1. 시스템을 재시작하여 환경 변수 변경사항 적용
2. 새 터미널/명령 프롬프트 창 열기
3. CGO 상태 확인:
```bash
go env CGO_ENABLED
```

4. 임시로 전역 CGO 설정:
```bash
go env -w CGO_ENABLED=1
```

## 빌드 방법

```bash
$env:CGO_ENABLED=1
go env -w CGO_ENABLED=1
go build -tags "windows" -ldflags "-H=windowsgui" -buildmode=exe
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
     "service_name": "hj-service",
    "service_description": "hj-service module",
    "restart_on_failure": true,
    "restart_delay": 5,
    "max_restart_attempts": 3,
    "log_path": ".\\logs",
    "database_path": ".\\db.sqlite",
    "monitoring_path": [
        "C:\\"
    ],
    "custom_data_path": ".\\data"
}
```

## 주의 사항

- 이 모듈은 Windows 운영체제에서만 작동합니다.
- 서비스 설치 및 관리는 관리자 권한이 필요합니다.
