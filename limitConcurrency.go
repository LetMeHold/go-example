package main

/*
使用带容量的channal限制最大并发数
*/

import (
	"log"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ { // 同时只有MAX个routine处于活动状态
		wg.Add(1)
		go func(i int) {
			defer limit()()
			defer wg.Done()
			time.Sleep(time.Second) // 活动状态下，每个routine需要一秒完成
			log.Printf("routine %d 的打印\n", i)
		}(i)
	}

	wg.Wait() // 所有routine完成需要 50 / 5 = 10 秒
	log.Println("over")
}

const MAX = 5

var ch = make(chan struct{}, MAX) // 容量即为并发的最大数量

func limit() func() {
	ch <- struct{}{}
	return func() {
		<-ch
	}
}
