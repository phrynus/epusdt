package main

// export CGO_ENABLED=0 && go build -ldflags="-s -w" -o epusdt .
// $env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -ldflags="-s -w" -o epusdt.exe .

import (
	"github.com/assimon/luuu/bootstrap"
	"github.com/assimon/luuu/config"
	"github.com/gookit/color"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			color.Error.Println("启动服务错误: ", err)
		}
	}()
	color.Infof("Epusdt 版本(%s) 作者: %s %s \n", config.GetAppVersion(), "assimon", "https://github.com/assimon/epusdt")
	bootstrap.Start()
}

// go run . http start
