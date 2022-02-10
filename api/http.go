package api

import (
	"encoding/json"
	"fmt"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/deepch/vdk/av"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	stream2 "github/zhangdapeng520/zdpgo_rtsp_to_webrtc/stream"
)

type JCodec struct {
	Type string
}

func ServeHTTP() {
	if g.Z == nil || g.G.App == nil {
		g.InitGlobal()
	}
	g.Z.Info("启动http服务")

	// 底层使用的是gin
	//gin.SetMode(gin.ReleaseMode)
	//
	//// 创建路由
	//router := gin.Default()
	//
	//// 使用跨域中间件
	//router.Use(CORSMiddleware())

	// 在web目录存在的条件下执行逻辑
	if _, err := os.Stat("./web"); !os.IsNotExist(err) {
		g.Z.Info("app是什么", "app", g.G.App)
		// 加载模板
		g.G.App.LoadHTMLGlob("web/templates/*")

		// GET方式的首页路由
		g.G.App.GET("/", HTTPAPIServerIndex)

		// GET方式的流媒体播放
		// TODO: 如何实现播放流的关键
		g.G.App.GET("/stream/player/:uuid", HTTPAPIServerStreamPlayer)

		// 新增一个流
		g.G.App.POST("/stream/player", HTTPAPIServerStreamPlayerAdd)

		// 删除一个流
		g.G.App.DELETE("/stream/player/:name", HTTPAPIServerStreamPlayerDelete)
	}

	g.G.App.POST("/stream/receiver/:uuid", HTTPAPIServerStreamWebRTC)

	// 通过uuid获取编码
	g.G.App.GET("/stream/codec/:uuid", HTTPAPIServerStreamCodec)

	g.G.App.POST("/stream", HTTPAPIServerStreamWebRTC2)

	g.G.App.StaticFS("/static", http.Dir("web/static"))
	g.Z.Info("即将启动服务", "port", stream2.Config.Server.HTTPPort)
	err := g.G.App.Run(stream2.Config.Server.HTTPPort)
	if err != nil {
		g.Z.Info("启动HTTP服务失败：", err)
	}
}

// HTTPAPIServerIndex  首页
func HTTPAPIServerIndex(c *gin.Context) {
	_, all := stream2.Config.List()
	if len(all) > 0 {
		c.Header("Cache-Control", "no-cache, max-age=0, must-revalidate, no-store")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Redirect(http.StatusMovedPermanently, "stream/player/"+all[0])
	} else {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":    stream2.Config.Server.HTTPPort,
			"version": time.Now().String(),
		})
	}
}

// HTTPAPIServerStreamPlayer 流媒体方法
func HTTPAPIServerStreamPlayer(c *gin.Context) {
	// 获取配置列表
	_, all := stream2.Config.List()

	// 以字符串的方式排序
	sort.Strings(all)

	// 返回一个HTML模板
	c.HTML(http.StatusOK, "player.tmpl", gin.H{
		"port":     stream2.Config.Server.HTTPPort, // 端口号
		"suuid":    c.Param("uuid"),                // uuid
		"suuidMap": all,                            // 所有的流
		"version":  time.Now().String(),            // 版本号
	})
}

// 添加流,确保RTSP流是唯一的
// 测试流：rtsp://admin:xx123456@111.198.61.222:9999/h264/ch1/main/av_stream
func HTTPAPIServerStreamPlayerAdd(c *gin.Context) {
	// 获取URL
	data := make(map[string]interface{}) // 注意该结构接受的内容
	c.BindJSON(&data)
	url := data["url"]
	fmt.Println("url:", url)
	fmt.Println("url:", data)

	// 判断该流是否已存在
	if ok, uuid := stream2.Config.IsExists(url.(string)); ok {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "该流已存在",
			"uuid": uuid,
		})
		return
	}

	// 生成流的名称
	name := g.R.RandomUUID()

	// 将流添加到streams
	stream := stream2.StreamST{
		URL:          url.(string),
		DisableAudio: true,
	}

	// 初始化map
	stream.Cl = make(map[string]stream2.Viewer)
	stream2.Config.Streams[name] = stream

	// 启动任务
	go stream2.RTSPWorkerLoop(name, stream.URL, stream.OnDemand, stream.DisableAudio, stream.Debug)

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
		"uuid": name,
	})
}

// 删除流
func HTTPAPIServerStreamPlayerDelete(c *gin.Context) {
	// 获取流的名称
	name := c.Param("name")
	fmt.Println("name:", name)

	delete(stream2.Config.Streams, name)

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
		"uuid": name,
	})
}

// HTTPAPIServerStreamCodec 流媒体编码
func HTTPAPIServerStreamCodec(c *gin.Context) {

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

// HTTPAPIServerStreamWebRTC 将视频流转换为WebRTC
func HTTPAPIServerStreamWebRTC(c *gin.Context) {

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

// 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, x-access-token")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

type Response struct {
	Tracks []string `json:"tracks"`
	Sdp64  string   `json:"sdp64"`
}

type ResponseError struct {
	Error string `json:"error"`
}

func HTTPAPIServerStreamWebRTC2(c *gin.Context) {
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
