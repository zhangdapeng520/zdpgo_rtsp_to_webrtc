package g

import (
	"github.com/zhangdapeng520/zdpgo_gin"
	"github.com/zhangdapeng520/zdpgo_random"
	"github.com/zhangdapeng520/zdpgo_signal"
	"github.com/zhangdapeng520/zdpgo_zap"
)

func InitGlobal() {
	initZap()
	initSignal()
	initRandom()
	initGin()
}

func initZap() {
	Z = zdpgo_zap.New(zdpgo_zap.ZapConfig{
		Debug:       true,
		OpenGlobal:  true,
		LogFilePath: "logs/zdpgo/zdpgo_rtsp_to_webrtc.log",
	})
}

func initSignal() {
	S = zdpgo_signal.New(zdpgo_signal.SignalConfig{})
}

func initRandom() {
	R = zdpgo_random.New(zdpgo_random.RandomConfig{
		Debug:       true,
		LogFilePath: "logs/zdpgo/zdpgo_random.log",
	})
}

func initGin() {
	G = zdpgo_gin.New(zdpgo_gin.GinConfig{
		Debug:       true,
		LogFilePath: "log/zdpgo/zdpgo_gin.log",
	})
}
