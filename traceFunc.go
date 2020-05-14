package main

/*
跟踪函数的进入、退出、持续时间。
*/

import (
	"log"
	"time"
)

func main() {
	sample()
}

func sample() {
	defer trace("func_sample")() // 别漏了()
	time.Sleep(time.Second * 3)
}

func trace(funcName string) func() {
	start := time.Now()
	log.Printf("enter %s", funcName)
	return func() {
		log.Printf("exit %s (duration %s)", funcName, time.Since(start))
	}
}
