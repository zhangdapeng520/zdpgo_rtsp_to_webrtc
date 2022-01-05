package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/deepch/vdk/codec/h264parser"

	"github.com/deepch/vdk/av"
)

// Config global
var Config = loadConfig()

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
	Cl           map[string]viewer // 连接
}

type viewer struct {
	c chan av.Packet
}

func (element *ConfigST) RunIFNotRun(uuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	if tmp, ok := element.Streams[uuid]; ok {
		if tmp.OnDemand && !tmp.RunLock {
			tmp.RunLock = true
			element.Streams[uuid] = tmp
			go RTSPWorkerLoop(uuid, tmp.URL, tmp.OnDemand, tmp.DisableAudio, tmp.Debug)
		}
	}
}

func (element *ConfigST) RunUnlock(uuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	if tmp, ok := element.Streams[uuid]; ok {
		if tmp.OnDemand && tmp.RunLock {
			tmp.RunLock = false
			element.Streams[uuid] = tmp
		}
	}
}

// 判断指定的视频流是否还有观看者
func (element *ConfigST) HasViewer(uuid string) bool {
	element.mutex.Lock()
	defer element.mutex.Unlock()

	// tmp.Cl：客户端连接数量
	if tmp, ok := element.Streams[uuid]; ok && len(tmp.Cl) > 0 {
		return true
	}
	return false
}

// 获取ICE服务列表
func (element *ConfigST) GetICEServers() []string {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.ICEServers
}

// 获取ICE用户名
func (element *ConfigST) GetICEUsername() string {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.ICEUsername
}

// 获取ICE协议
func (element *ConfigST) GetICECredential() string {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.ICECredential
}

// 获取webrtc的最小端口号
func (element *ConfigST) GetWebRTCPortMin() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMin
}

// 获取webrtc的最大端口号
func (element *ConfigST) GetWebRTCPortMax() uint16 {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	return element.Server.WebRTCPortMax
}

// 加载配置
func loadConfig() *ConfigST {
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
			v.Cl = make(map[string]viewer)
			tmp.Streams[i] = v
		}
	} else {
		// 读取配置文件出错，从命令行提取数据
		// addr := flag.String("listen", "8083", "HTTP host:port")
		port := os.Args[1]
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		log.Println("服务地址：", fmt.Sprintf("http://%s", addr))
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

// 抓包
func (element *ConfigST) cast(uuid string, pck av.Packet) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	for _, v := range element.Streams[uuid].Cl {
		if len(v.c) < cap(v.c) {
			v.c <- pck
		}
	}
}

// 判断该流媒体是否存在
func (element *ConfigST) ext(suuid string) bool {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	_, ok := element.Streams[suuid]
	return ok
}

// 添加媒体流
func (element *ConfigST) coAd(suuid string, codecs []av.CodecData) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	t := element.Streams[suuid]
	t.Codecs = codecs
	element.Streams[suuid] = t
}

// 获取媒体流
func (element *ConfigST) coGe(suuid string) []av.CodecData {
	for i := 0; i < 100; i++ {
		element.mutex.RLock()
		tmp, ok := element.Streams[suuid]
		element.mutex.RUnlock()
		if !ok {
			return nil
		}
		if tmp.Codecs != nil {
			// TODO Delete test
			for _, codec := range tmp.Codecs {
				if codec.Type() == av.H264 {
					codecVideo := codec.(h264parser.CodecData)
					if codecVideo.SPS() != nil && codecVideo.PPS() != nil && len(codecVideo.SPS()) > 0 && len(codecVideo.PPS()) > 0 {
						// ok
						// log.Println("Ok Video Ready to play")
					} else {
						// video codec not ok
						log.Println("Bad Video Codec SPS or PPS Wait")
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

// 建立流媒体通信管道
func (element *ConfigST) clAd(suuid string) (string, chan av.Packet) {
	// 加锁
	element.mutex.Lock()

	// 释放锁
	defer element.mutex.Unlock()

	// 获取uuid
	cuuid := pseudoUUID()

	// 创建管道
	ch := make(chan av.Packet, 100)

	// 添加到流
	element.Streams[suuid].Cl[cuuid] = viewer{c: ch}

	// 返回管道id和管道对象
	return cuuid, ch
}

// 获取流媒体列表
func (element *ConfigST) list() (string, []string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	var res []string
	var fist string
	for k := range element.Streams {
		if fist == "" {
			fist = k
		}
		res = append(res, k)
	}
	return fist, res
}

// 删除一个流媒体
func (element *ConfigST) clDe(suuid, cuuid string) {
	element.mutex.Lock()
	defer element.mutex.Unlock()
	delete(element.Streams[suuid].Cl, cuuid)
}

// 生成一个uuid
func pseudoUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return
}
