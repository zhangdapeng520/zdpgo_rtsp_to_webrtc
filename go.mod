module zdpgo_rtsp_to_webrtc

go 1.16

require (
	github.com/deepch/vdk v0.0.0-20210508200759-5adbbcc01f89
	github.com/gin-gonic/gin v1.7.0
	github.com/zhangdapeng520/zdpgo_random v0.1.0
	github.com/zhangdapeng520/zdpgo_signal v0.1.0
	github.com/zhangdapeng520/zdpgo_zap v0.2.1
)

replace github.com/zhangdapeng520/zdpgo_signal v0.1.0 => ../zdpgo_signal
replace github.com/zhangdapeng520/zdpgo_random v0.1.0 => ../zdpgo_random
