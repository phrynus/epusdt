package main

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

// garble build -o epusdt_mix.exe -trimpath -ldflags="-s -w"
// go run . http start
