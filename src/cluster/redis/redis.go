package redis

import (
	"fmt"
	"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	"time"
)

//Creator redis创建器
type Creator struct {
	SessionID    int           `json:"sessionID"`
	Cpu          int           `json:"cpu"`
	Mem          int           `json:"mem"`
	Namespace    string        `json:"namespace"`
	ReplicaCount int           `json:"replicaCount"`
	Timeout      time.Duration `json:"timeout"` //单位秒 (int64)
}

//OptCMD 安装操作命令
type OptCMD struct {
	DockerCMDList []cmd.ExecInfo `json:"dockerCMDList"`
	YamlCMDList   []cmd.ExecInfo `json:"yamlCMDList"`
	CreateCMDList []cmd.ExecInfo `json:"createCMDList"`
}

const (
	redisYamlName    = "/redis.yaml"
	redisYamlRecvDir = "/opt/redis"
)

//PushDockerImage 上传docker镜像
func (c *Creator) PushDockerImage(dockerRegistryURL string) error {
	return nil
}

//ModifyYaml 修改yaml文件
func (c *Creator) ModifyYaml(dockerRegistryURL, k8sAPIServerURL string) error {

	return nil
}

//Build 构建redis组件
func (c *Creator) Build(k8sMasterSSH kubessh.LoginInfo) error {
	logdebug.Println(logdebug.LevelInfo, "---构建redis---", *c)

	yamlFile := config.GetDockerImagesTarPath() + redisYamlName

	sshClient, err := k8sMasterSSH.ConnectToHost()
	if err != nil {
		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	mkdirCMD := fmt.Sprintf(`sudo mkdir -p %s`, redisYamlRecvDir)

	kubessh.ClientRunCMD(c.SessionID, sshClient, cmd.ExecInfo{CMDContent: mkdirCMD})

	//发送到对端/opt目录
	//kubessh.SftpSend(sftpClient, yamlFile, destRecvDir)
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, redisYamlRecvDir)

	url := "http://" + k8sMasterSSH.HostAddr + ":" + config.GetWorkerPort() + "/flash/jobs/exec"
	createCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`kubectl create -f %s`, redisYamlRecvDir),
		EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
	}

	req := msg.Request{
		URL:     url,
		Type:    msg.POST,
		Content: createCMD,
	}

	_, err = req.SendRequestByJSON()
	if err != nil {
		logdebug.Println(logdebug.LevelError, "创建redis失败:", err.Error())
	}

	return err
}
