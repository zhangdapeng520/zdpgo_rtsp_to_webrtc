package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 开启HTTP服务
	go serveHTTP()

	// 开启服务流
	go serveStreams()

	// 优雅退出：监听服务退出信号
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println(sig)
		done <- true
	}()
	log.Println("系统运行中，等待退出信号")
	<-done
	log.Println("退出系统")
}
