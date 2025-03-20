//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// ServiceConfig는 서비스 설정 정보를 담는 구조체
type ServiceConfig struct {
	ServiceName        string `json:"service_name"`
	ServiceDescription string `json:"service_description"`
	// 서비스 재시작 정책 설정
	RestartOnFailure   bool `json:"restart_on_failure"`
	RestartDelay       int  `json:"restart_delay"` // 초 단위
	MaxRestartAttempts int  `json:"max_restart_attempts"`
	// 로그 설정
	LogPath string `json:"log_path"`
	// 데이터베이스 경로 설정
	DatabasePath string `json:"database_path"`
	// 모니터링 경로 설정
	MonitoringPath []string `json:"monitoring_path"`
	// 기타 설정
	CustomDataPath string `json:"custom_data_path"`
}

// 기본 설정값
var defaultConfig = ServiceConfig{
	ServiceName:        "hj-service",
	ServiceDescription: "hj-service module",
	RestartOnFailure:   true,
	RestartDelay:       5,
	MaxRestartAttempts: 3,
	LogPath:            "./logs",
	DatabasePath:       "./db.sqlite",
	MonitoringPath:     []string{"C:\\"},
	CustomDataPath:     "./data",
}

// LoadConfig는 설정 파일을 읽어옵니다.
func LoadConfig(configPath string) (*ServiceConfig, error) {
	var config ServiceConfig

	// 설정 파일이 존재하지 않으면 기본 설정 값 반환
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &defaultConfig, nil
	}

	// 설정 파일 읽기
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// JSON 파싱
	err = json.Unmarshal(configData, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig는 설정을 파일에 저장합니다.
func SaveConfig(config *ServiceConfig, configPath string) error {
	// 디렉토리가 없으면 생성
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	// JSON 직렬화
	configData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	// 파일 저장
	return os.WriteFile(configPath, configData, 0644)
}

// EnsureDefaultConfig는 설정 파일이 없으면 기본 설정을 저장합니다.
func EnsureDefaultConfig(configPath string) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err = SaveConfig(&defaultConfig, configPath)
		if err != nil {
			log.Printf("기본 설정 파일 생성 실패: %v", err)
		} else {
			log.Printf("기본 설정 파일이 생성되었습니다: %s", configPath)
		}
	}
}
