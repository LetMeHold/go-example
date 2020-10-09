package main

/*
监听（tail -f）以小时分隔的nginx日志，并将内容发送到MultiPartFile的http接口
*/

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/hpcloud/tail"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

var (
	conf     Config
	app      []byte
	muWhence sync.Mutex
	tmot     time.Duration
	tmFmt    = "2006010215"
	tConf    = tail.Config{
		Follow: true,
		Location: &tail.SeekInfo{
			Offset: 0,
			Whence: 2, // 0 文件开头, 1 指定Offset, 2 文件末尾
		},
	}
	tBox   = TailBox{box: make(map[string]*tail.Tail)}
	errLog = log.New(os.Stdout, "ERROR ", log.Ldate|log.Ltime)
)

func main() {
	fname := "config.json"
	err := loadConf(fname, &conf)
	if err != nil {
		errLog.Printf("载入%s失败：%v", fname, err)
		return
	}

	t := time.NewTicker(time.Second)
OutFor:
	for { // 为避免切换文件时上一个文件有遗留数据来不及处理, 默认启动秒数为30
		select {
		case <-t.C:
			if time.Now().Second() != conf.StartSecond {
				log.Printf("未到启动秒数: %d, 等待...", conf.StartSecond)
			} else {
				break OutFor
			}
		}
	}
	t.Stop()

	log.Println("程序准备完成，开始启动。")
	for _, proj := range conf.Files {
		path := conf.Path + "/" + proj
		if _, e := os.Stat(path); e != nil {
			if os.IsNotExist(e) {
				errLog.Printf("路径 %s 不存在!", path)
				continue
			}
		}
		go manageTail(path)
	}

	tc := time.NewTicker(time.Minute * 15)
	sChan := make(chan os.Signal)
	signal.Notify(sChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	for {
		select {
		case <-tc.C:
			log.Printf("当前线程数: %d, 监听文件数: %d", runtime.NumGoroutine(), tBox.len())
		case s := <-sChan:
			log.Println("接收到退出信号: ", s)
			tBox.clear()
		}
	}
}

type Config struct {
	Url          string   `json: "Url"`
	Timeout      int      `json: "Timeout"`
	LineNum      int      `json: "LineNum"`
	StartSecond  int      `json: "StartSecond"`
	FirstWhence  int      `json: "FirstWhence"`
	FollowWhence int      `json: "FollowWhence"`
	App          string   `json: "App"`
	Path         string   `json: "Path"`
	Files        []string `json: "Files"`
}

func loadConf(fname string, conf *Config) error {
	contents, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}
	err = json.Unmarshal(contents, &conf)
	if err != nil {
		return err
	}
	app = []byte(conf.App)
	tmot = time.Duration(conf.Timeout)
	tConf.Location.Whence = conf.FirstWhence
	return nil
}

func manageTail(path string) {
	defer traceMT(path)()
	recvBuf := &bytes.Buffer{}
	sendBuf := &bytes.Buffer{}
	for {
		tm := time.Now().Format(tmFmt)
		filename := path + "/access-" + tm + ".log"
		t, e := tail.TailFile(filename, tConf)
		if e != nil {
			errLog.Printf("%s tail faild: %v", filename, e)
			return
		}
		recvTail(t, recvBuf, sendBuf)
	}
}

func traceMT(path string) func() {
	log.Printf("启动监听线程: %s", path)
	return func() {
		log.Printf("停止监听线程: %s", path)
	}
}

func recvTail(t *tail.Tail, recvBuf, sendBuf *bytes.Buffer) {
	defer traceRT(t)()
	start := time.Now()
	tc := time.NewTicker(time.Minute)
	count := 0
	total := 0
	success := 0
	lineSend := 0
	timeSend := 0
OutFor:
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				errLog.Printf("%s tail chan 已被关闭!", t.Filename)
				break OutFor
			}
			recvBuf.WriteString(line.Text)
			recvBuf.WriteString("\n")
			count++
			total++
			if count == conf.LineNum { // 缓存指定行数后一起发送
				lineSend += 1
				e := send(recvBuf, sendBuf, t, count)
				if e != nil {
					errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", t.Filename, count, e)
				} else {
					success += count
				}
				count = 0
			}
		case <-tc.C:
			if count > 0 { // 超过一定时间，没达到指定行数也要发送
				timeSend += 1
				e := send(recvBuf, sendBuf, t, count)
				if e != nil {
					errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", t.Filename, count, e)
				} else {
					success += count
				}
				count = 0
			}
			if time.Now().Hour() != start.Hour() {
				// 到达下一个小时，本次监听完成使命，进入manageTail的下一个循环
				break OutFor
			}
		}
	}
	tc.Stop()
	log.Printf("%s 读取行数: %d, 发送行数: %d, 满足行数发送%d次, 满足超时发送%d次", t.Filename, total, success, lineSend, timeSend)
}

func traceRT(t *tail.Tail) func() {
	log.Printf("开始监听文件: %s", t.Filename)
	tBox.add(t)
	return func() {
		tBox.del(t)
		if tConf.Location.Whence != conf.FollowWhence {
			// 默认首次启动从文件末尾tail，后续则从文件开头tail
			muWhence.Lock()
			tConf.Location.Whence = conf.FollowWhence
			muWhence.Unlock()
		}
		log.Printf("停止监听文件: %s", t.Filename)
	}
}

func send(recvBuf, sendBuf *bytes.Buffer, t *tail.Tail, count int) error {
	defer recvBuf.Reset()
	defer sendBuf.Reset()
	writer := multipart.NewWriter(sendBuf)
	part1, _ := writer.CreateFormFile("log", t.Filename)
	_, e1 := part1.Write(recvBuf.Bytes())
	if e1 != nil {
		writer.Close()
		return e1
	}
	part2, _ := writer.CreateFormField("app")
	_, e5 := part2.Write(app)
	if e5 != nil {
		writer.Close()
		return e5
	}

	contentType := writer.FormDataContentType()
	writer.Close()
	req, e2 := http.NewRequest("POST", conf.Url, sendBuf)
	if e2 != nil {
		return e2
	}
	req.Header.Set("Content-Type", contentType)
	client := &http.Client{Timeout: time.Duration(time.Second * tmot)}
	rep, e3 := client.Do(req)

	if e3 != nil {
		return e3
	}
	body, e4 := ioutil.ReadAll(rep.Body)
	rep.Body.Close()
	if e4 != nil {
		return e4
	}
	ret := string(body)
	if ret != "{\"code\":\"0000\"}" {
		return errors.New(ret)
	}
	return nil
}

type TailBox struct {
	box map[string]*tail.Tail
	mu  sync.Mutex
}

func (tb *TailBox) add(t *tail.Tail) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.box[t.Filename] = t
}

func (tb *TailBox) del(t *tail.Tail) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.closeTail(t, "TailBox.del")
	delete(tb.box, t.Filename)
}

func (tb *TailBox) len() int {
	return len(tb.box)
}

func (tb *TailBox) clear() {
	now := time.Now()
	if now.Minute() == 0 && now.Second() == conf.StartSecond {
		// 等待文件切换时间过去
		time.Sleep(time.Second * 1)
	}
	tb.mu.Lock()
	for _, t := range tb.box {
		tb.closeTail(t, "TailBox.clear")
	}
	time.Sleep(time.Second * 1)
	log.Printf("程序清理完成（%d个文件监听），正常退出。", len(tb.box))
	tb.mu.Unlock()
	os.Exit(0)
}

func (tb *TailBox) closeTail(t *tail.Tail, location string) {
	log.Printf("%s stop tail on %s 开始", t.Filename, location)
	if e := t.Stop(); e != nil {
		errLog.Printf("%s stop tail on %s 出现错误: %v", t.Filename, location, e)
	}
	log.Printf("%s stop tail on %s 结束", t.Filename, location)
	log.Printf("%s cleanup tail on %s 开始", t.Filename, location)
	t.Cleanup()
	log.Printf("%s cleanup tail on %s 结束", t.Filename, location)
}
