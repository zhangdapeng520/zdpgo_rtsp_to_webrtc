package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	stream2 "github/zhangdapeng520/zdpgo_rtsp_to_webrtc/stream"
	"net/http"
	"sort"
	"time"
)

// getStreamPlayer 流媒体方法
func getStreamPlayer(c *gin.Context) {
	// 获取配置列表
	_, all := stream2.Config.List()

	// 以字符串的方式排序
	sort.Strings(all)

	// 返回一个HTML模板
	c.HTML(http.StatusOK, "player.html", gin.H{
		"port":     stream2.Config.Server.HTTPPort, // 端口号
		"suuid":    c.Param("uuid"),                // uuid
		"suuidMap": all,                            // 所有的流
		"version":  time.Now().String(),            // 版本号
	})
}

// postStreamPlayer 添加流,确保RTSP流是唯一的
// postStreamPlayer 测试流：rtsp://admin:xx123456@111.198.61.222:9999/h264/ch1/main/av_stream
func postStreamPlayer(c *gin.Context) {
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

// deleteStreamPlayer 删除流
func deleteStreamPlayer(c *gin.Context) {
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
