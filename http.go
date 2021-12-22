package main

import (
	"encoding/json"
	`fmt`
	"log"
	"net/http"
	"os"
	"sort"
	"time"
	
	"github.com/deepch/vdk/av"
	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
)

type JCodec struct {
	Type string
}

func serveHTTP() {
	// 底层使用的是gin
	gin.SetMode(gin.ReleaseMode)
	
	// 创建路由
	router := gin.Default()
	
	// 使用跨域中间件
	router.Use(CORSMiddleware())
	
	// 在web目录存在的条件下执行逻辑
	if _, err := os.Stat("./web"); !os.IsNotExist(err) {
		// 加载模板
		router.LoadHTMLGlob("web/templates/*")
		
		// GET方式的首页路由
		router.GET("/", HTTPAPIServerIndex)
		
		// GET方式的流媒体播放
		// TODO: 如何实现播放流的关键
		router.GET("/stream/player/:uuid", HTTPAPIServerStreamPlayer)
		
		// 新增一个流
		router.POST("/stream/player/:name", HTTPAPIServerStreamPlayerAdd)
		
		// 删除一个流
		router.DELETE("/stream/player/:name", HTTPAPIServerStreamPlayerDelete)
	}
	
	router.POST("/stream/receiver/:uuid", HTTPAPIServerStreamWebRTC)
	
	// 通过uuid获取编码
	router.GET("/stream/codec/:uuid", HTTPAPIServerStreamCodec)
	
	router.POST("/stream", HTTPAPIServerStreamWebRTC2)
	
	router.StaticFS("/static", http.Dir("web/static"))
	err := router.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatalln("Start HTTP Server error", err)
	}
}

// HTTPAPIServerIndex  index
func HTTPAPIServerIndex(c *gin.Context) {
	_, all := Config.list()
	if len(all) > 0 {
		c.Header("Cache-Control", "no-cache, max-age=0, must-revalidate, no-store")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Redirect(http.StatusMovedPermanently, "stream/player/"+all[0])
	} else {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"port":    Config.Server.HTTPPort,
			"version": time.Now().String(),
		})
	}
}

// HTTPAPIServerStreamPlayer 流媒体方法
func HTTPAPIServerStreamPlayer(c *gin.Context) {
	// 获取配置列表
	_, all := Config.list()
	
	// 以字符串的方式排序
	sort.Strings(all)
	
	// 返回一个HTML模板
	c.HTML(http.StatusOK, "player.tmpl", gin.H{
		"port":     Config.Server.HTTPPort, // 端口号
		"suuid":    c.Param("uuid"),        // uuid
		"suuidMap": all,                    // 所有的流
		"version":  time.Now().String(),    // 版本号
	})
}

// 添加流
func HTTPAPIServerStreamPlayerAdd(c *gin.Context) {
	// 获取流的名称
	name := c.Param("name")
	fmt.Println("name:", name)
	
	// 获取URL
	data := make(map[string]interface{}) // 注意该结构接受的内容
	c.BindJSON(&data)
	url := data["url"]
	fmt.Println("url:", url)
	fmt.Println("url:", data)
	
	// 将流添加到streams
	stream := StreamST{
		URL:          url.(string),
		DisableAudio: true,
	}
	
	// 初始化map
	stream.Cl = make(map[string]viewer)
	Config.Streams[name] = stream
	
	// 启动任务
	go RTSPWorkerLoop(name, stream.URL, stream.OnDemand, stream.DisableAudio, stream.Debug)
	
	// 重定向
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
	})
}

// 删除流
func HTTPAPIServerStreamPlayerDelete(c *gin.Context) {
	// 获取流的名称
	name := c.Param("name")
	fmt.Println("name:", name)
	
	delete(Config.Streams, name)
	
	// 重定向
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
	})
}

// HTTPAPIServerStreamCodec 流媒体编码
func HTTPAPIServerStreamCodec(c *gin.Context) {
	
	// 如果存在该UUID
	if Config.ext(c.Param("uuid")) {
		
		// 运行该流媒体
		Config.RunIFNotRun(c.Param("uuid"))
		
		// 获取编码
		codecs := Config.coGe(c.Param("uuid"))
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
	if !Config.ext(c.PostForm("suuid")) {
		log.Println("视频流不存在")
		return
	}
	
	// 运行提交的指定视频流
	Config.RunIFNotRun(c.PostForm("suuid"))
	
	// 获取编码
	codecs := Config.coGe(c.PostForm("suuid"))
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
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{ICEServers: Config.GetICEServers(), ICEUsername: Config.GetICEUsername(), ICECredential: Config.GetICECredential(), PortMin: Config.GetWebRTCPortMin(), PortMax: Config.GetWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, c.PostForm("data"))
	if err != nil {
		log.Println("WriteHeader", err)
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		log.Println("写入WebRTC失败：", err)
		return
	}
	
	// 开启新的协程任务
	go func() {
		// 创建流媒体管道
		cid, ch := Config.clAd(c.PostForm("suuid"))
		defer Config.clDe(c.PostForm("suuid"), cid)
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
	if _, ok := Config.Streams[url]; !ok {
		Config.Streams[url] = StreamST{
			URL:      url,
			OnDemand: true,
			Cl:       make(map[string]viewer),
		}
	}
	
	Config.RunIFNotRun(url)
	
	codecs := Config.coGe(url)
	if codecs == nil {
		log.Println("Stream Codec Not Found")
		c.JSON(500, ResponseError{Error: Config.LastError.Error()})
		return
	}
	
	muxerWebRTC := webrtc.NewMuxer(
		webrtc.Options{
			ICEServers: Config.GetICEServers(),
			PortMin:    Config.GetWebRTCPortMin(),
			PortMax:    Config.GetWebRTCPortMax(),
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
		cid, ch := Config.clAd(url)
		defer Config.clDe(url, cid)
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
