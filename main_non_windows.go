//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("이 도구는 Windows 운영체제에서만 사용할 수 있습니다.")
	fmt.Println("이 모듈은 Windows 서비스를 관리하기 위한 것입니다.")
	os.Exit(1)
}
