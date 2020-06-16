package main

/*
用flag解析参数示例
*/

import (
	"flag"
	"fmt"
	"strings"
)

func main() {
	// 解析命令行参数
	b := flag.Bool("b", false, "Is ok ?")
	s := flag.String("s", "null", "Print message .")
	flag.Parse()
	fmt.Println(*b, *s, flag.Args())

	// 解析字符串
	myflag := flag.NewFlagSet("myflag", flag.ContinueOnError)
	b = myflag.Bool("b", false, "Is ok ?")
	s = myflag.String("s", "null", "Print message .")
	cmd := "-b -s message Other contents"
	if err := myflag.Parse(strings.Fields(cmd)); err == nil {
		fmt.Println(*b, *s, myflag.Args())
	}
}
