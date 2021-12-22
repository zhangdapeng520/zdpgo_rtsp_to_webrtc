package main

import (
	"errors"
	"log"
	"time"

	"github.com/deepch/vdk/format/rtspv2"
)

// 异常错误常量
var (
	ErrorStreamExitNoVideoOnStream = errors.New("流退出：该流上不存在视频")
	ErrorStreamExitRtspDisconnect  = errors.New("流退出：RTSP断开连接")
	ErrorStreamExitNoViewer        = errors.New("流退出：守护进程上没有观看者了")
)

// 开启流服务
func serveStreams() {
	// 遍历所有的流
	for k, v := range Config.Streams {
		// 如果没有启动
		if !v.OnDemand {
			// 启动流
			go RTSPWorkerLoop(k, v.URL, v.OnDemand, v.DisableAudio, v.Debug)
		}
	}
}

// 启动RTSP工作流
func RTSPWorkerLoop(name, url string, OnDemand, DisableAudio, Debug bool) {
	// 延迟解锁
	defer Config.RunUnlock(name)
	for {
		log.Println("尝试建立连接：", name)

		// 开始工作
		err := RTSPWorker(name, url, OnDemand, DisableAudio, Debug)
		if err != nil {
			log.Println(err)
			Config.LastError = err
		}
		if OnDemand && !Config.HasViewer(name) {
			log.Println(ErrorStreamExitNoViewer)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// RTSP工作进程
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
		Config.coAd(name, RTSPClient.CodecData)
	}

	// 是否为纯音频
	var AudioOnly bool
	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
		AudioOnly = true
	}

	// 监听不同的goroutine数据
	for {
		select {
		case <-clientTest.C:
			if OnDemand {
				if !Config.HasViewer(name) {
					return ErrorStreamExitNoViewer
				} else {
					clientTest.Reset(20 * time.Second)
				}
			}
		case <-keyTest.C:
			return ErrorStreamExitNoVideoOnStream
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Config.coAd(name, RTSPClient.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return ErrorStreamExitRtspDisconnect
			}
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if AudioOnly || packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
			}
			Config.cast(name, *packetAV)
		}
	}
}
