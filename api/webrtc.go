package api

import (
	"encoding/json"
	"fmt"
	"github.com/deepch/vdk/av"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	stream2 "github/zhangdapeng520/zdpgo_rtsp_to_webrtc/stream"
	"log"
	"time"
)

// postStreamWebRTC 将视频流转换为WebRTC
func postStreamWebRTC(c *gin.Context) {

	// 优先考虑边界条件
	if !stream2.Config.Ext(c.PostForm("suuid")) {
		log.Println("视频流不存在")
		return
	}

	// 运行提交的指定视频流
	stream2.Config.RunIFNotRun(c.PostForm("suuid"))

	// 获取编码
	codecs := stream2.Config.CoGe(c.PostForm("suuid"))
	if codecs == nil {
		log.Println("不存在该视频流的编码信息")
		return
	}

	// 是否自动播放
	var AudioOnly bool
	if len(codecs) == 1 && codecs[0].Type().IsAudio() {
		AudioOnly = true
	}

	// 转换为WebRTC
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{ICEServers: stream2.Config.GetICEServers(), ICEUsername: stream2.Config.GetICEUsername(), ICECredential: stream2.Config.GetICECredential(), PortMin: stream2.Config.GetWebRTCPortMin(), PortMax: stream2.Config.GetWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, c.PostForm("data"))
	if err != nil {
		log.Println("写入WebRTC头失败", err)
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		log.Println("写入WebRTC answer失败：", err)
		return
	}

	// 开启新的协程任务
	go func() {
		// 创建流媒体管道
		uuid := c.PostForm("suuid")
		cid, ch := stream2.Config.ClAd(uuid)
		defer stream2.Config.ClDe(uuid, cid)
		defer muxerWebRTC.Close()

		// 是否开始播放视频
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				log.Println("没有视频")
				return
			case pck := <-ch:
				if pck.IsKeyFrame || AudioOnly {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart && !AudioOnly {
					continue
				}
				err = muxerWebRTC.WritePacket(pck)
				if err != nil {
					stream2.Config.ClDe(uuid, cid) // 断开当前的连接

					// TODO: 断开连接以后，一定会走这个方法
					// TODO: 查看该name的数量，如果已经是0了，则删除
					fmt.Println("uuid是什么：", uuid)
					fmt.Println("是否还有人在连接：", stream2.Config.HasViewer(uuid))
					tmp, ok := stream2.Config.Streams[uuid]
					fmt.Println("ok", ok)
					fmt.Println("tmp.Cl", tmp.Cl, len(tmp.Cl))

					if !stream2.Config.HasViewer(uuid) {
						delete(stream2.Config.Streams, uuid)
					}
					log.Println("写入packet信息失败：", err)
					return
				}
			}
		}
	}()
}

// getStreamCodec 流媒体编码
func getStreamCodec(c *gin.Context) {

	// 如果存在该UUID
	if stream2.Config.Ext(c.Param("uuid")) {

		// 运行该流媒体
		stream2.Config.RunIFNotRun(c.Param("uuid"))

		// 获取编码
		codecs := stream2.Config.CoGe(c.Param("uuid"))
		if codecs == nil {
			return
		}

		// 临时的编码
		var tmpCodec []JCodec
		for _, codec := range codecs {
			// 如果不是能够编码的类型
			if codec.Type() != av.H264 && codec.Type() != av.PCM_ALAW && codec.Type() != av.PCM_MULAW && codec.Type() != av.OPUS {
				log.Println("Codec 不支持 WebRTC，忽略此轨道", codec.Type())
				continue
			}

			// 编码视频类型
			if codec.Type().IsVideo() {
				tmpCodec = append(tmpCodec, JCodec{Type: "video"})

				// 编码非视频类型
			} else {
				tmpCodec = append(tmpCodec, JCodec{Type: "audio"})
			}
		}

		// 转换为json编码
		b, err := json.Marshal(tmpCodec)
		if err == nil {
			_, err = c.Writer.Write(b)
			if err != nil {
				log.Println("写入流媒体编码信息错误：", err)
				return
			}
		}
	}
}

func postStreamWebRTC2(c *gin.Context) {
	url := c.PostForm("url")
	if _, ok := stream2.Config.Streams[url]; !ok {
		stream2.Config.Streams[url] = stream2.StreamST{
			URL:      url,
			OnDemand: true,
			Cl:       make(map[string]stream2.Viewer),
		}
	}

	stream2.Config.RunIFNotRun(url)

	codecs := stream2.Config.CoGe(url)
	if codecs == nil {
		log.Println("Stream Codec Not Found")
		c.JSON(500, ResponseError{Error: stream2.Config.LastError.Error()})
		return
	}

	muxerWebRTC := webrtc.NewMuxer(
		webrtc.Options{
			ICEServers: stream2.Config.GetICEServers(),
			PortMin:    stream2.Config.GetWebRTCPortMin(),
			PortMax:    stream2.Config.GetWebRTCPortMax(),
		},
	)

	sdp64 := c.PostForm("sdp64")
	answer, err := muxerWebRTC.WriteHeader(codecs, sdp64)
	if err != nil {
		log.Println("Muxer WriteHeader", err)
		c.JSON(500, ResponseError{Error: err.Error()})
		return
	}

	response := Response{
		Sdp64: answer,
	}

	for _, codec := range codecs {
		if codec.Type() != av.H264 &&
			codec.Type() != av.PCM_ALAW &&
			codec.Type() != av.PCM_MULAW &&
			codec.Type() != av.OPUS {
			log.Println("Codec Not Supported WebRTC ignore this track", codec.Type())
			continue
		}
		if codec.Type().IsVideo() {
			response.Tracks = append(response.Tracks, "video")
		} else {
			response.Tracks = append(response.Tracks, "audio")
		}
	}

	c.JSON(200, response)

	AudioOnly := len(codecs) == 1 && codecs[0].Type().IsAudio()

	go func() {
		cid, ch := stream2.Config.ClAd(url)
		defer stream2.Config.ClDe(url, cid)
		defer muxerWebRTC.Close()
		var videoStart bool
		noVideo := time.NewTimer(10 * time.Second)
		for {
			select {
			case <-noVideo.C:
				log.Println("noVideo")
				return
			case pck := <-ch:
				if pck.IsKeyFrame || AudioOnly {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart && !AudioOnly {
					continue
				}
				err = muxerWebRTC.WritePacket(pck)
				if err != nil {
					log.Println("WritePacket", err)
					return
				}
			}
		}
	}()
}
