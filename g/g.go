package g

import (
	"github.com/zhangdapeng520/zdpgo_random"
	"github.com/zhangdapeng520/zdpgo_signal"
	"github.com/zhangdapeng520/zdpgo_zap"
)

var (
	Z *zdpgo_zap.Zap       // zap日志核心对象
	S *zdpgo_signal.Signal // signal信号核心对象
	R *zdpgo_random.Random // random随机数核心对象
)
