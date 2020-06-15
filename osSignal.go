package main

/*
处理系统发来的退出信号
*/

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	t := time.NewTicker(time.Second)
	c := make(chan os.Signal)
	signal.Notify(c)
	for {
		select {
		case <-t.C:
			log.Println("程序运行中...")
		case s := <-c:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP:
				log.Println("接收到退出信号: ", s)
				os.Exit(0)
			default:
				log.Println("接收到其他信号: ", s)
			}
		}
	}
}
