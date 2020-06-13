package main

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	//client, err := NewConnByPwd("172.18.180.110:22", "root", "password")
	//client, err := NewConnByKey("172.18.180.110:22", "root")
	client, err := NewConnByKeyWithPwd("172.18.180.110:22", "root", "/root/.ssh/id_rsa_pwd", "key_password")
	if err != nil {
		log.Fatalf("Create new connect failed : %v", err)
	}
	if err := Commond(client, "locale"); err != nil {
		log.Fatalf("Commond failed : %v", err)
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

// Commond 在远程主机执行一条命令
func Commond(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	exe := "source /etc/profile;" + cmd // non-login形式默认不读/etc/profile
	if err := session.Run(exe); err != nil {
		return err
	}
	return nil
}
