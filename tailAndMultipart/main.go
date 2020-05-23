package main

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
	"time"
)

var conf Config
var app []byte
var tmot time.Duration

func main() {
	fname := "config.json"
	err := loadConf(fname, &conf)
	if err != nil {
		log.Printf("载入%s失败：%v", fname, err)
		return
	}
	app = []byte(conf.App)
	tmot = time.Duration(conf.Timeout)

	for _, proj := range conf.Files {
		path := conf.Path + "/" + proj
		if _, e := os.Stat(path); e != nil {
			if os.IsNotExist(e) {
				log.Printf("路径 %s 不存在!", path)
				continue
			}
		}
		go manageTail(path)
	}

	tc := time.NewTicker(time.Minute * 15)
	for {
		select {
		case <-tc.C:
			log.Printf("Goroutine number: %d", runtime.NumGoroutine())
		}
	}
}

type Config struct {
	Url     string   `json: "Url"`
	Timeout int      `json: "Timeout"`
	LineNum int      `json: "LineNum"`
	App     string   `json: "App"`
	Path    string   `json: "Path"`
	Files   []string `json: "Files"`
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
	return nil
}

var tConf = tail.Config{
	Follow: true,
	Location: &tail.SeekInfo{
		Offset: 0,
		Whence: 2, // 0 文件开头, 1 指定Offset, 2 文件末尾
	},
}

var tmFmt = "2006010215"

func manageTail(path string) {
	for {
		tm := time.Now().Format(tmFmt)
		filename := path + "/access-" + tm + ".log"
		t, e := tail.TailFile(filename, tConf)
		if e != nil {
			log.Printf("%s tail faild: %v", filename, e)
			return
		}
		recvTail(t)
	}
}

func recvTail(t *tail.Tail) {
	log.Printf("开始监听文件: %s", t.Filename)
	start := time.Now()

	tc := time.NewTicker(time.Minute)
	var count int
	data := &bytes.Buffer{}

OutFor:
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				log.Printf("%s tail chan 出现未知错误!", t.Filename)
				break OutFor
			}
			data.WriteString(line.Text)
			data.WriteString("\n")
			count++
			if count == conf.LineNum { // 缓存指定行数后一起发送
				send(data, t.Filename)
				data = &bytes.Buffer{}
				count = 0
			}
		case <-tc.C:
			if count > 0 { // 超过一定时间，没达到指定行数也要发送
				send(data, t.Filename)
				data = &bytes.Buffer{}
				count = 0
			}
			if time.Now().Hour() != start.Hour() {
				// 到达下一个小时，本次监听完成使命，进入manageTail的下一个循环
				break OutFor
			}
		}
	}
	t.Cleanup()
	t.Stop()
	tConf.Location.Whence = 0 // 首次启动从文件末尾开始，后面则从文件开头
	log.Printf("停止监听文件: %s", t.Filename)
}

func send(data *bytes.Buffer, filename string) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	part1, _ := writer.CreateFormFile("log", filename)
	_, e1 := part1.Write(data.Bytes())
	if e1 != nil {
		log.Printf("%s 发送数据失败: %v", filename, e1)
		writer.Close()
		return
	}
	part2, _ := writer.CreateFormField("app")
	_, e5 := part2.Write(app)
	if e5 != nil {
		log.Printf("%s 发送数据失败: %v", filename, e5)
		writer.Close()
		return
	}

	contentType := writer.FormDataContentType()
	writer.Close()
	req, e2 := http.NewRequest("POST", conf.Url, buf)
	if e2 != nil {
		log.Printf("%s 发送数据失败: %v", filename, e2)
		return
	}
	req.Header.Set("Content-Type", contentType)
	client := &http.Client{Timeout: time.Duration(time.Second * tmot)}
	rep, e3 := client.Do(req)

	if e3 != nil {
		log.Printf("%s 发送数据失败: %v", filename, e3)
		return
	}
	body, e4 := ioutil.ReadAll(rep.Body)
	rep.Body.Close()
	if e4 != nil {
		log.Printf("%s 发送数据失败: %v", filename, e4)
		return
	}
	ret := string(body)
	if ret != "{\"code\":\"0000\"}" {
		log.Printf("%s 发送数据失败: %s", filename, ret)
	}
}
