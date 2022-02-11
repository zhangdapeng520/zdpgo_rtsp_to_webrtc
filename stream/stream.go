package stream

import (
	"errors"
	"github.com/deepch/vdk/format/rtspv2"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	"time"
)

// 异常错误常量
var (
	ErrorStreamExitNoVideoOnStream = errors.New("流退出：该流上不存在视频")
	ErrorStreamExitRtspDisconnect  = errors.New("流退出：RTSP断开连接")
	ErrorStreamExitNoViewer        = errors.New("流退出：守护进程上没有观看者了")
)

// ServeStreams 开启流服务
func ServeStreams() {
	// 遍历所有的流
	for k, v := range Config.Streams {
		if !v.OnDemand { // 如果没有启动
			go RTSPWorkerLoop(k, v.URL, v.OnDemand, v.DisableAudio, v.Debug) // 启动流
		}
	}
}

// RTSPWorkerLoop 启动RTSP工作流
// name：流的uuid，也是流的名称
// url：流的rtspUrl字符串
// OnDemand：是否未启用
// DisableAudio：是否禁用音频
// Debug：是否为调试模式
func RTSPWorkerLoop(name, url string, OnDemand, DisableAudio, Debug bool) {
	// 延迟解锁
	defer Config.RunUnlock(name)
	for {
		err := RTSPWorker(name, url, OnDemand, DisableAudio, Debug) // 启动RTSP工作者
		if err != nil {
			g.Z.Error("执行RTSPWorker错误", "error", err)
			Config.LastError = err
		}

		// 监听浏览器的退出信号
		tmp, ok := Config.Streams[name]
		if ok {
			g.Z.Info("当前视频流的客户端连接数", "数量", len(tmp.Cl), "客户端列表", tmp.Cl)
		}
		if OnDemand && !Config.HasViewer(name) { // 如果是未启用的，且该流没有观看者了
			g.Z.Error("流的所有客户端都关闭了", "error", ErrorStreamExitNoViewer)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// RTSPWorker RTSP工作进程
// name：rtsp流的uuid
// url：rtsp流的url
// OnDemand：是否未启用
// DisableAudio：是否禁用音频
// Debug：是否为调试模式
func RTSPWorker(name, url string, OnDemand, DisableAudio, Debug bool) error {

	keyTest := time.NewTimer(20 * time.Second)
	clientTest := time.NewTimer(20 * time.Second)

	// 添加下一次超时
	// 创建RTSP客户端
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: url, DisableAudio: DisableAudio, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: Debug})
	if err != nil {
		return err
	}
	defer RTSPClient.Close()

	// 添加解码数据
	if RTSPClient.CodecData != nil {
		Config.CoAd(name, RTSPClient.CodecData)
	}

	// 是否为纯音频
	var AudioOnly bool
	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
		AudioOnly = true
	}

	// 监听不同的goroutine数据
	for {
		select {

		// 测试客户端
		case <-clientTest.C:
			if OnDemand {
				if !Config.HasViewer(name) {
					return ErrorStreamExitNoViewer
				} else {
					clientTest.Reset(20 * time.Second)
				}
			}

		// 测试按键
		case <-keyTest.C:
			return ErrorStreamExitNoVideoOnStream
		// 信号
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Config.CoAd(name, RTSPClient.CodecData)
			case rtspv2.SignalStreamRTPStop:
				g.Z.Info("流媒体断开连接")
				return ErrorStreamExitRtspDisconnect
			}
		// 打包
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if AudioOnly || packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
			}
			Config.Cast(name, *packetAV)
		}
	}
}
