package main

/*
监听（tail -f）以小时分隔的nginx日志，并将内容发送到MultiPartFile的http接口
*/

import (
	"bytes"
	"encoding/json"
	"github.com/hpcloud/tail"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	conf  Config
	app   []byte
	mu    sync.Mutex
	tmot  time.Duration
	tmFmt = "2006010215"
	tConf = tail.Config{
		Follow: true,
		Location: &tail.SeekInfo{
			Offset: 0,
			Whence: 2, // 0 文件开头, 1 指定Offset, 2 文件末尾
		},
	}
	infLog = log.New(os.Stdout, "INFO ", log.Ldate|log.Ltime)
	errLog = log.New(os.Stdout, "ERROR ", log.Ldate|log.Ltime)
)

func main() {
	fname := "config.json"
	err := loadConf(fname, &conf)
	if err != nil {
		errLog.Printf("载入%s失败：%v", fname, err)
		return
	}

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
	for {
		select {
		case <-tc.C:
			infLog.Printf("Goroutine number: %d", runtime.NumGoroutine())
		}
	}
}

type Config struct {
	Url          string   `json: "Url"`
	Timeout      int      `json: "Timeout"`
	LineNum      int      `json: "LineNum"`
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
	infLog.Printf("启动监听线程: %s", path)
	return func() {
		infLog.Printf("停止监听线程: %s", path)
	}
}

func recvTail(t *tail.Tail, recvBuf, sendBuf *bytes.Buffer) {
	defer traceRT(t)()
	start := time.Now()
	tc := time.NewTicker(time.Minute)
	count := 0
OutFor:
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				errLog.Printf("%s tail chan 出现未知错误!", t.Filename)
				break OutFor
			}
			recvBuf.WriteString(line.Text)
			recvBuf.WriteString("\n")
			count++
			if count == conf.LineNum { // 缓存指定行数后一起发送
				send(recvBuf, sendBuf, t.Filename, count)
				count = 0
			}
		case <-tc.C:
			if count > 0 { // 超过一定时间，没达到指定行数也要发送
				send(recvBuf, sendBuf, t.Filename, count)
				count = 0
			}
			if time.Now().Hour() != start.Hour() {
				// 到达下一个小时，本次监听完成使命，进入manageTail的下一个循环
				break OutFor
			}
		}
	}
}

func traceRT(t *tail.Tail) func() {
	infLog.Printf("开始监听文件: %s", t.Filename)
	return func() {
		t.Cleanup()
		if e := t.Stop(); e != nil {
			errLog.Printf("%s stop tail 出现错误: %v", t.Filename, e)
		}
		if tConf.Location.Whence != conf.FollowWhence {
			// 默认首次启动从文件末尾tail，后续则从文件开头tail
			mu.Lock()
			tConf.Location.Whence = conf.FollowWhence
			mu.Unlock()
		}
		infLog.Printf("停止监听文件: %s", t.Filename)
	}
}

func send(recvBuf, sendBuf *bytes.Buffer, filename string, count int) {
	defer recvBuf.Reset()
	defer sendBuf.Reset()
	writer := multipart.NewWriter(sendBuf)
	part1, _ := writer.CreateFormFile("log", filename)
	_, e1 := part1.Write(recvBuf.Bytes())
	if e1 != nil {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", filename, count, e1)
		writer.Close()
		return
	}
	part2, _ := writer.CreateFormField("app")
	_, e5 := part2.Write(app)
	if e5 != nil {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", filename, count, e5)
		writer.Close()
		return
	}

	contentType := writer.FormDataContentType()
	writer.Close()
	req, e2 := http.NewRequest("POST", conf.Url, sendBuf)
	if e2 != nil {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", filename, count, e2)
		return
	}
	req.Header.Set("Content-Type", contentType)
	client := &http.Client{Timeout: time.Duration(time.Second * tmot)}
	rep, e3 := client.Do(req)

	if e3 != nil {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", filename, count, e3)
		return
	}
	body, e4 := ioutil.ReadAll(rep.Body)
	rep.Body.Close()
	if e4 != nil {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %v", filename, count, e4)
		return
	}
	ret := string(body)
	if ret != "{\"code\":\"0000\"}" {
		errLog.Printf("%s 发送数据失败，丢弃日志%d行: %s", filename, count, ret)
	}
}
