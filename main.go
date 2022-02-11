package main

import (
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/api"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/stream"
)

func init() {
	g.InitGlobal() // 初始化全局变量
}

func main() {
	go api.ServeHTTP()       // 开启HTTP服务
	go stream.ServeStreams() // 开启服务流
	g.S.Exit()               // 优雅退出：监听服务退出信号
}
