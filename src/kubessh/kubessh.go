package kubessh

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/node"
	"kubeinstall/src/state"
	//"log"
	"kubeinstall/src/msg"
	"net"
	"os"
	"path"
	"strings"
	"time"
)

//LoginInfo ssh登录信息
type LoginInfo struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
	HostAddr string `json:"hostAddr"`
	Port     int    `json:"port"`
}

const (
	sshCMDOutputFilePath = "/tmp/sshcmd.output"
	sshTimeout           = 30
)

//ConnectToHost 创建ssh连接 ssh客户端
func (sshUser *LoginInfo) ConnectToHost() (*ssh.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(sshUser.Password))

	clientConfig = &ssh.ClientConfig{
		User:    sshUser.UserName,
		Auth:    auth,
		Timeout: sshTimeout * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", sshUser.HostAddr, sshUser.Port)

	client, err = ssh.Dial("tcp", addr, clientConfig)

	return client, err
}

func SftpDownload(sftpClient *sftp.Client, remoteFilePath string, localDir string) error {
	var (
		err error
	)

	logdebug.Println(logdebug.LevelInfo, "-----sftp 下载文件-----", remoteFilePath)

	// 用来测试的远程文件路径 和 本地文件夹
	//var remoteFilePath = "/home/a.txt"
	//var localDir = "./"

	srcFile, err := sftpClient.Open(remoteFilePath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return err
	}
	defer srcFile.Close()

	var localFileName = path.Base(remoteFilePath)
	dstFile, err := os.Create(path.Join(localDir, localFileName))
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return err
	}
	defer dstFile.Close()

	if _, err = srcFile.WriteTo(dstFile); err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return err
	}

	fmt.Println("copy file from remote server finished!")

	return nil
}

//ExecSendFileCMD 执行向对端发送文件命令(sftp 到指定的目录 在cp过去)
func ExecSendFileCMD(sshClient *ssh.Client, sftpClient *sftp.Client, srcfilePath string, destDir string) error {
	//userRecvPath := config.GetDestReceiverPath()
	userCacheDir := config.GetCacheDir()
	var fileName = path.Base(srcfilePath)

	if userCacheDir == destDir {
		logdebug.Println(logdebug.LevelError, "缓存目录与目标接收目录相同")

		return errors.New("传输失败")
	}

	SftpSend(sftpClient, srcfilePath, userCacheDir)

	session, err := sshClient.NewSession()
	if err != nil {
		return err
	}

	defer session.Close()

	f, _ := os.Create(sshCMDOutputFilePath)

	defer f.Close()

	//重定向结果至文件，读取 检查......
	session.Stdout = f
	session.Stderr = f

	//cp到对端指定DIR
	//打印一份在kubeinstall的log中
	//logdebug.Println(logdebug.LevelInfo, "---ssh尝试执行命令---", cmd)

	copyCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`sudo cp %s %s`, userCacheDir+fileName, destDir),
	}
	logdebug.Println(logdebug.LevelInfo, "---ssh尝试执行命令---", copyCMD)

	err = session.Run(copyCMD.CMDContent)

	//发送一份给用户
	output, _ := ioutil.ReadFile(sshCMDOutputFilePath)
	//
	//if err != nil {
	//	logdebug.Println(logdebug.LevelInfo, "---ssh尝试执行命令---", copyCMD.CMDContent, "执行结果---", string(output))
	//
	//	return err
	//}

	err = checkOutput(string(output), copyCMD)

	//打印一份在kubeinstall的log中
	logdebug.Println(logdebug.LevelInfo, string(output))

	return err
}

//SftpSend 远程发送
func SftpSend(sftpClient *sftp.Client, srcfilePath string, destDir string) error {
	var (
		err error
	)

	logdebug.Println(logdebug.LevelInfo, "-----sftp 发送文件-----", srcfilePath, ":", destDir)

	// 用来测试的本地文件路径 和 远程机器上的文件夹
	var localFilePath = srcfilePath
	var remoteDir = destDir
	srcFile, err := os.Open(localFilePath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return err
	}
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)
	dstFile, err := sftpClient.Create(path.Join(remoteDir, remoteFileName))
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return err
	}
	defer dstFile.Close()

	f, _ := srcFile.Stat()

	buf := make([]byte, f.Size())
	//buf := make([]byte, 1024)
	for {
		n, _ := srcFile.Read(buf)
		if n == 0 {
			break
		}
		dstFile.Write(buf)
	}

	//fmt.Println("copy file to remote server finished!")

	return nil
}

func cutIPFromHostAddr(hostAddr string) string {
	body := strings.Split(hostAddr, ":")

	return body[0]
}

//ClientRunCMD 执行命令
func ClientRunCMD(sessionID int, client *ssh.Client, cmd cmd.ExecInfo) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}

	context := state.Context{
		State:           state.Running,
		SchedulePercent: cmd.SchedulePercent,
	}

	ip := cutIPFromHostAddr(client.RemoteAddr().String())

	cmdPrefix := fmt.Sprintf(`[%s]# `, ip)
	//原始命令 先写入一个命令body 以便前端知道当前卡在那个命令
	context.Stdout = cmdPrefix + cmd.CMDContent + "\n"
	context.State = state.Running
	context.Tips = cmd.Tips
	state.Update(sessionID, context)

	req := msg.Request{
		URL:  "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + ip,
		Type: msg.PUT,
		Content: node.NodeContext{
			Context: state.Context{
				Stdout: cmdPrefix + cmd.CMDContent + "\n",
				State:  state.Running,
				Tips:   cmd.Tips,
			},
		},
	}

	req.SendRequestByJSON()

	//fmt.Println("-----newSession----", err)

	defer session.Close()

	f, _ := os.Create(sshCMDOutputFilePath)

	//重定向结果至文件，读取 检查......
	session.Stdout = f
	session.Stderr = f

	defer f.Close()

	//打印一份在kubeinstall的log中
	//logdebug.Println(logdebug.LevelInfo, "---ssh尝试执行命令---", cmd)
	//err = session.Run(cmd.CMDContent)

	session.Run(cmd.CMDContent)
	//session.Run(cmd.CMDContent)

	//发送一份给用户
	output, _ := ioutil.ReadFile(sshCMDOutputFilePath)

	//if err != nil {
	//	logdebug.Println(logdebug.LevelInfo, "---ssh尝试执行命令---", cmd, "执行结果---", err, string(output))
	//	//context.ErrMsg = err.Error()
	//	//context.State = state.Error
	//	//context.Tips += "...安装失败!"
	//	//req.Content = node.NodeContext{
	//	//	Context: state.Context{
	//	//		Stdout: string(output),
	//	//		State:  state.Error,
	//	//		Tips:   context.Tips,
	//	//	},
	//	//}
	//	//
	//	//req.SendRequestByJSON()
	//	//
	//	//state.Update(sessionID, context)
	//
	//	//return string(output), err
	//}

	err = checkOutput(string(output), cmd)
	context.Stdout = string(output)
	req.Content = node.NodeContext{
		Context: state.Context{
			Stdout: string(output),
		},
	}
	if err != nil {
		context.ErrMsg = err.Error()
		context.State = state.Error
		context.Tips += "...安装失败!"
		req.Content = node.NodeContext{
			Context: state.Context{
				Stdout: string(output),
				State:  state.Error,
				Tips:   context.Tips,
			},
		}
	}

	req.SendRequestByJSON()

	state.Update(sessionID, context)

	//打印一份在kubeinstall的log中
	logdebug.Println(logdebug.LevelInfo, string(output))

	return string(output), err
}

func checkOutput(output string, cmd cmd.ExecInfo) error {
	//先检查错误点
	for _, errKey := range cmd.ErrorTags {
		if strings.Contains(output, errKey) {
			logdebug.Println(logdebug.LevelInfo, "--------命令输出检查报错------", cmd)
			return errors.New("命令输出检查报错")
		}
	}

	for _, successKey := range cmd.SuccessTags {
		if !strings.Contains(output, successKey) {
			logdebug.Println(logdebug.LevelInfo, "--------命令输出检查不成功------", cmd)
			return errors.New("命令输出检查不成功")
		}
	}

	logdebug.Println(logdebug.LevelInfo, "--------命令执行成功------", cmd.CMDContent)

	return nil
}
