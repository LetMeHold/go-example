package main

/*
等待所有并发协程的结束
*/

import (
	"log"
	"sync"
	"time"
)

func main() {
	ch := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(i) * time.Second)
            log.Printf("routine %d 的打印\n", i)
			ch <- i
		}(i)
	}

	go func() {
		wg.Wait()
		close(ch)
		log.Println("所有routine结束")
	}()

	log.Println("主线程开始接受数据")
	for i := range ch {
		log.Printf("收到 routine %d 的信息", i)
	}
}
