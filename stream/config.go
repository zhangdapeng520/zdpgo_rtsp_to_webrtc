package stream

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github/zhangdapeng520/zdpgo_rtsp_to_webrtc/g"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	Config = loadConfig()
)

// 加载配置
func loadConfig() *ConfigST {
	if g.Z == nil {
		g.InitGlobal()
	}
	g.Z.Info("加载配置。。。")

	// 临时的配置对象
	var tmp ConfigST

	// 读取配置
	data, err := ioutil.ReadFile("config.json")
	if err == nil {

		// 解析配置
		err = json.Unmarshal(data, &tmp)
		if err != nil {
			log.Fatalln(err)
		}

		// 生成流map
		for i, v := range tmp.Streams {
			v.Cl = make(map[string]Viewer)
			tmp.Streams[i] = v
		}
	} else {
		// 读取配置文件出错，从命令行提取数据
		// addr := flag.String("listen", "8083", "HTTP host:port")
		port := 8888
		if len(os.Args) > 1 {
			port, _ = strconv.Atoi(os.Args[1])
		}
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		log.Println("服务地址：", addr)
		udpMin := flag.Int("udp_min", 0, "WebRTC UDP port min")
		udpMax := flag.Int("udp_max", 0, "WebRTC UDP port max")
		// iceServer := flag.String("ice_server", "", "ICE Server")
		iceServer := "stun:stun.l.google.com:19302"
		flag.Parse()

		// 将提取的数据添加到配置中
		tmp.Server.HTTPPort = addr
		tmp.Server.WebRTCPortMin = uint16(*udpMin)
		tmp.Server.WebRTCPortMax = uint16(*udpMax)
		// if len(*iceServer) > 0 {
		// 	tmp.Server.ICEServers = []string{*iceServer}
		// }
		tmp.Server.ICEServers = []string{iceServer}

		// 创建一个空的stream map
		tmp.Streams = make(map[string]StreamST)
	}

	// 返回临时的配置
	return &tmp
}

// ConfigST 配置对象
type ConfigST struct {
	mutex     sync.RWMutex
	Server    ServerST            `json:"server"`  // 服务配置
	Streams   map[string]StreamST `json:"streams"` // 流配置
	LastError error               // 最后的错误
}

// ServerST 服务的相关配置
type ServerST struct {
	HTTPPort      string   `json:"http_port"`       // 端口号
	ICEServers    []string `json:"ice_servers"`     // ice服务列表
	ICEUsername   string   `json:"ice_username"`    // ice用户名
	ICECredential string   `json:"ice_credential"`  // ice证书
	WebRTCPortMin uint16   `json:"webrtc_port_min"` // webtrc最小端口号
	WebRTCPortMax uint16   `json:"webrtc_port_max"` // webrtc最大端口号
}

// StreamST 流媒体相关配置
type StreamST struct {
	URL          string            `json:"url"`    // 路径
	Status       bool              `json:"status"` // 状态
	OnDemand     bool              `json:"on_demand"`
	DisableAudio bool              `json:"disable_audio"` // 禁用音频
	Debug        bool              `json:"debug"`         // debug模式
	RunLock      bool              `json:"-"`             // 运行锁
	Codecs       []av.CodecData    // 编码
	Cl           map[string]Viewer // 客户端连接
}

// Viewer 客户端，观看者，视频用户
type Viewer struct {
	c chan av.Packet
}

// RunIFNotRun 运行uuid对应的流
func (element *ConfigST) RunIFNotRun(uuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 取出uuid对应的流
	if tmp, ok := element.Streams[uuid]; ok {
		if tmp.OnDemand && !tmp.RunLock { // 如果流死了，但是没有加锁
			tmp.RunLock = true                                                          // 加锁
			element.Streams[uuid] = tmp                                                 // 更新
			go RTSPWorkerLoop(uuid, tmp.URL, tmp.OnDemand, tmp.DisableAudio, tmp.Debug) // 运行
		}
	}
}

// RunUnlock 释放锁
func (element *ConfigST) RunUnlock(uuid string) {
	// 锁的添加和释放
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 执行业务逻辑
	// 取出uuid对应的流
	if tmp, ok := element.Streams[uuid]; ok { // 如果该流已经死掉了，而且该流仍处于加锁状态
		if tmp.OnDemand && tmp.RunLock {
			tmp.RunLock = false         // 释放锁
			element.Streams[uuid] = tmp // 重新赋值，即修改
		}
	}
}

// IsExists 判断指定的rtspUrl是否已存在
func (element *ConfigST) IsExists(rtspUrl string) (bool, string) {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 遍历流，根据rtspUrl寻找流
	for uuid, v := range element.Streams {
		if v.URL == rtspUrl {
			return true, uuid // 找到了，返回true和该流对应的uuid
		}
	}
	return false, "" // 没找到
}

// HasViewer 判断指定的视频流是否还有观看者
func (element *ConfigST) HasViewer(uuid string) bool {
	// 加锁和延迟释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// tmp.Cl：客户端连接数量
	if tmp, ok := element.Streams[uuid]; ok && len(tmp.Cl) > 0 { // 找到该流，且该流的客户端连接数大于0
		return true
	}
	return false
}

// GetICEServers 获取ICE服务列表
func (element *ConfigST) GetICEServers() []string {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 获取配置中的ICE服务列表
	return element.Server.ICEServers
}

// GetICEUsername 获取ICE用户名
func (element *ConfigST) GetICEUsername() string {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 返回服务的ICE用户名
	return element.Server.ICEUsername
}

// GetICECredential 获取ICE协议
func (element *ConfigST) GetICECredential() string {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 返回服务的ice协议
	return element.Server.ICECredential
}

// GetWebRTCPortMin 获取webrtc的最小端口号
func (element *ConfigST) GetWebRTCPortMin() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMin
}

// GetWebRTCPortMax 获取webrtc的最大端口号
func (element *ConfigST) GetWebRTCPortMax() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMax
}

// Cast 抓包
func (element *ConfigST) Cast(uuid string, pck av.Packet) {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 遍历uui的的客户端列表
	for _, v := range element.Streams[uuid].Cl {
		if len(v.c) < cap(v.c) {
			v.c <- pck
		}
	}
}

// Ext 判断该流媒体是否存在
func (element *ConfigST) Ext(suuid string) bool {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	_, ok := element.Streams[suuid]
	return ok
}

// CoAd 给指定流添加codecs编码
func (element *ConfigST) CoAd(suuid string, codecs []av.CodecData) {
	element.mutex.Lock()
	defer element.mutex.Unlock()

	t := element.Streams[suuid] // 取出流
	t.Codecs = codecs           // 指定编码
	element.Streams[suuid] = t  // 更新
}

// CoGe 获取媒体流
func (element *ConfigST) CoGe(suuid string) []av.CodecData {
	for i := 0; i < 100; i++ { // 遍历100次
		element.mutex.RLock()
		tmp, ok := element.Streams[suuid] // 取出流
		element.mutex.RUnlock()
		if !ok {
			return nil
		}

		// 存在codecs
		if tmp.Codecs != nil {
			// TODO Delete test
			for _, codec := range tmp.Codecs {
				if codec.Type() == av.H264 {
					codecVideo := codec.(h264parser.CodecData)
					if codecVideo.SPS() != nil && codecVideo.PPS() != nil && len(codecVideo.SPS()) > 0 && len(codecVideo.PPS()) > 0 {
						// ok
						g.Z.Info("视频解析成功")
					} else {
						// video codec not ok
						log.Println("视频解析失败")
						time.Sleep(50 * time.Millisecond)
						continue
					}
				}
			}
			return tmp.Codecs
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// ClAd 建立流媒体通信管道
func (element *ConfigST) ClAd(suuid string) (string, chan av.Packet) {
	// 加锁和释放锁
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 获取uuid
	cuuid := g.R.RandomUUID()

	// 创建管道
	ch := make(chan av.Packet, 100)

	// 添加到流
	element.Streams[suuid].Cl[cuuid] = Viewer{c: ch}

	// 返回管道id和管道对象
	return cuuid, ch
}

// List 获取流媒体列表
func (element *ConfigST) List() (string, []string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// 资源列表和rtspUrl字符串
	var res []string
	var fist string

	// 遍历所有的流
	for k := range element.Streams {
		if fist == "" {
			fist = k
		}
		res = append(res, k)
	}
	return fist, res
}

// ClDe 删除一个流媒体
func (element *ConfigST) ClDe(suuid, cuuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	delete(element.Streams[suuid].Cl, cuuid)
}

// 生成一个uuid
//func pseudoUUID() (uuid string) {
//	b := make([]byte, 16)
//	_, err := rand.Read(b)
//	if err != nil {
//		fmt.Println("Error: ", err)
//		return
//	}
//	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
//	return
//}
