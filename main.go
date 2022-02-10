package main

import (
	"zdpgo_rtsp_to_webrtc/api"
	"zdpgo_rtsp_to_webrtc/g"
	"zdpgo_rtsp_to_webrtc/stream"
)

func init() {
	g.InitGlobal() // 初始化全局变量
}

func main() {
	// 开启HTTP服务
	go api.ServeHTTP()

	// 开启服务流
	go stream.ServeStreams()

	// 优雅退出：监听服务退出信号
	g.S.Exit()
}
