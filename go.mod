module windows_service_module

go 1.24.0

require (
	github.com/yhj0901/windowsIOMonitoring v0.1.4
	golang.org/x/sys v0.31.0
)

require (
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.24 // indirect
)

// 로컬 개발 시 아래 replace 구문을 해제하세요
// replace github.com/yhj0901/windowsIOMonitoring => ../windowsIOMonitoring
