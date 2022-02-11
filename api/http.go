package api

import (
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	stream2 "github/zhangdapeng520/zdpgo_rtsp_to_webrtc/stream"
)

type JCodec struct {
	Type string
}

// ServeHTTP 启动http服务
func ServeHTTP() {
	if g.Z == nil || g.G.App == nil {
		g.InitGlobal()
	}
	g.Z.Info("启动http服务")
	g.Z.Info("app是什么", "app", g.G.App)

	// 在web目录存在的条件下执行逻辑
	if _, err := os.Stat("./web"); !os.IsNotExist(err) {
		g.G.App.GET("/", index)                                    // GET方式的首页路由
		g.G.App.GET("/stream/player/:uuid", getStreamPlayer)       // 获取流
		g.G.App.POST("/stream/player", postStreamPlayer)           // 新增一个流
		g.G.App.DELETE("/stream/player/:name", deleteStreamPlayer) // 删除一个流
	}

	// webrtc相关路由
	{
		g.G.App.POST("/stream/receiver/:uuid", postStreamWebRTC) // 创建接收者
		g.G.App.GET("/stream/codec/:uuid", getStreamCodec)       // 通过uuid获取编码
		g.G.App.POST("/stream", postStreamWebRTC2)               // 创建流
	}

	// 启动服务
	g.Z.Info("即将启动服务", "port", stream2.Config.Server.HTTPPort)
	err := g.G.App.Run(stream2.Config.Server.HTTPPort)
	if err != nil {
		g.Z.Info("启动HTTP服务失败", "error", err)
	}
}

// index  首页
func index(c *gin.Context) {
	// 获取流列表
	_, all := stream2.Config.List()
	g.Z.Info("流列表", "all", all)

	// 存在流
	if len(all) > 0 {
		c.Header("Cache-Control", "no-cache, max-age=0, must-revalidate, no-store")
		c.Header("Access-Control-Allow-Origin", "*")
		// 重定向到播放流的页面
		g.Z.Info("重定向到播放流的页面", "url", "stream/player/"+all[0])
		c.Redirect(http.StatusMovedPermanently, "stream/player/"+all[0])
	} else {
		// 渲染首页
		g.Z.Info("c.HTML", "c.HTML", c.HTML)
		c.HTML(http.StatusOK, "index.html", gin.H{
			"port":    stream2.Config.Server.HTTPPort,
			"version": time.Now().String(),
		})
	}
}
