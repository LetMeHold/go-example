package main

/*
自定义一个本地终端
*/

import (
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"log"
	"os"
	"strings"
)

var autoGroup = []string{
	"hello, world",
	"hello, china",
	"golang",
	"goto",
}

func main() {
	fd := int(os.Stdin.Fd())
	state, er := terminal.MakeRaw(fd)
	if er != nil {
		log.Fatal(er)
	}
	defer terminal.Restore(fd, state)
	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}

	term := terminal.NewTerminal(screen, "[MyTerm]$ ")
	term.AutoCompleteCallback =
		func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
			if key != 9 { // 非tab键不处理
				return "", 0, false
			}
			if pos == 0 {
				new := strings.Join(autoGroup, "\t")
				term.Write([]byte(new + "\n"))
				return "", 0, false
			}
			var tmp []string
			for _, v := range autoGroup {
				if strings.HasPrefix(v, line) {
					tmp = append(tmp, v)
				}
			}
			if len(tmp) == 0 {
				return line, pos, false
			} else if len(tmp) == 1 {
				return tmp[0], len(tmp[0]), true
			}
			new := strings.Join(tmp, "\t")
			term.Write([]byte(new + "\n"))
			return "", 0, false
		}

	for {
		line, err := term.ReadLine()
		if err != nil {
			log.Fatal(err)
		}
		if line == "" {
			continue
		}
		if line == "exit" {
			break
		}
		term.Write([]byte(line + "\n"))
	}
}
