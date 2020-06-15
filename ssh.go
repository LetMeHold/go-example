package main

/*
通过ssh远程执行命令
*/

import (
	"bytes"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"log"
	"os"
	//"bufio"
)

func main() {
	//client, err := NewConnByPwd("172.18.180.110:22", "root", "password")
	client, err := NewConnByKey("172.18.180.110:22", "root")
	//client, err := NewConnByKeyWithPwd("172.18.180.110:22", "root", "/root/.ssh/id_rsa_pwd", "key_password")
	if err != nil {
		log.Fatalf("Create new connect failed : %v", err)
	}
	defer client.Close()
	if err := Run(client, "whoami"); err != nil {
		log.Fatalf("Run failed : %v", err)
	}
	if err := Start(client, "ping 127.0.0.1 -c 3"); err != nil {
		log.Fatalf("Start failed : %v", err)
	}
    // Shell在Start或Shell后启动，第一下键盘操作会不起作用，尚未找到解决方法
	if err := Shell(client); err != nil {
		log.Fatalf("Start shell failed : %v", err)
	}
}

// NewConnByPwd 通过用户名密码建立连接
func NewConnByPwd(host, usr, pwd string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: usr,
		Auth: []ssh.AuthMethod{
			ssh.Password(pwd),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 接受所有hostkey
		/*
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil  // 判断hostkey, 返回nil表示接受
			},
		*/
	}
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// NewConnByKey 通过免密key建立连接
func NewConnByKey(host, usr string) (*ssh.Client, error) {
	var keyFile string
	if usr == "root" {
		keyFile = "/root/.ssh/id_rsa"
	} else {
		keyFile = "/home/" + usr + "/.ssh/id_rsa"
	}
	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: usr,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 接受所有hostkey
	}
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// NewConnByKeyWithPwd 通过带密码的key建立连接
func NewConnByKeyWithPwd(host, usr, keyFile, keyPwd string) (*ssh.Client, error) {
	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(keyPwd))
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: usr,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 接受所有hostkey
	}
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Run 在远程主机执行一条命令，并获取输出
func Run(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf
	exe := "source /etc/profile;" + cmd // non-login形式默认不读/etc/profile
	if err := session.Run(exe); err != nil {
	    log.Printf("%s error :\n%s", cmd, errBuf.String())
		return err
	}
	log.Printf("%s output :\n%s", cmd, outBuf.String())
	return nil
}

// Start 在远程主机执行一条需要交互或持续输出的命令，比如ssh-keygen和tail -f
func Start(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	exe := "source /etc/profile;" + cmd // non-login形式默认不读/etc/profile
	err = session.Start(exe)
	if err != nil {
		return err
	}
	err = session.Wait()
	if err != nil {
		return err
	}
	return nil
}

// Shell 从远程主机获取一个完整的shell会话
func Shell(client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	// 获取本机终端信息
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, state)
	termWidth, termHeight, err := terminal.GetSize(fd)
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	// 通过本机信息获取伪终端
	err = session.RequestPty(termType, termHeight, termWidth, ssh.TerminalModes{})
	if err != nil {
		return err
	}
	// 启动终端，期间不要改变本机终端的大小，否则会显示错位
	err = session.Shell()
	if err != nil {
		return err
	}
	err = session.Wait()
	if err != nil {
		return err
	}
	return nil
}
