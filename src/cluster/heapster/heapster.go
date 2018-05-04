package heapster

import (
	"fmt"
	"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	"kubeinstall/src/runconf"
	"strconv"
	"strings"
	"time"
)

//Creator heapster创建器
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
	heapsterYaml        = "/heapster.yaml"
	heapsterRbacYaml    = "/heapster-rbac.yaml"
	heapsterYamlRecvDir = "/opt/heapster"
)

func (c *Creator) modifyCMD(src []cmd.ExecInfo, dockerRegistryURL, k8sAPIServerURL string) {
	dockerImagesPath := config.GetDockerImagesTarPath()
	cpu := strconv.Itoa(c.Cpu)
	mem := strconv.Itoa(c.Mem)

	for k := range src {
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DOCKER_IMAGES_PATH)", dockerImagesPath, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DOCKER_REGISTRY_URL)", dockerRegistryURL, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(CPU)", cpu, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(MEM)", mem, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(K8S_API_URL)", k8sAPIServerURL, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEST_RECV_DIR)", heapsterYamlRecvDir, -1)
	}

	return
}

//PushDockerImage 上传docker镜像
func (c *Creator) PushDockerImage(dockerRegistryURL string) error {
	optCMD := OptCMD{}

	err := runconf.Read(runconf.DataTypeHeapsterCMDConf, &optCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---读取heapster命令json文件失败---", err.Error())

		return err
	}

	c.modifyCMD(optCMD.DockerCMDList, dockerRegistryURL, "")
	logdebug.Println(logdebug.LevelInfo, "---读取heapster命令json文件内容---", optCMD.DockerCMDList)

	//调试时 push太慢 暂时注释
	for _, v := range optCMD.DockerCMDList {
		v.Exec()
	}

	return nil
}

//ModifyYaml 修改yaml文件
func (c *Creator) ModifyYaml(dockerRegistryURL, k8sAPIServerURL string) error {
	optCMD := OptCMD{}

	err := runconf.Read(runconf.DataTypeHeapsterCMDConf, &optCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---读取heapster命令json文件失败---", err.Error())

		return err
	}

	c.modifyCMD(optCMD.YamlCMDList, dockerRegistryURL, k8sAPIServerURL)

	logdebug.Println(logdebug.LevelInfo, "---读取heapster命令json文件内容---", optCMD.YamlCMDList)

	for _, v := range optCMD.YamlCMDList {
		v.Exec()
	}

	return nil
}

//Build 构建heapster组件
func (c *Creator) Build(k8sMasterSSH kubessh.LoginInfo) error {
	logdebug.Println(logdebug.LevelInfo, "---构建heapster---", *c)

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

	//modifyYamlFile()

	mkdirCMD := fmt.Sprintf(`sudo mkdir -p %s`, heapsterYamlRecvDir)

	kubessh.ClientRunCMD(c.SessionID, sshClient, cmd.ExecInfo{CMDContent: mkdirCMD})

	//发送到对端/opt目录
	yamlFile := config.GetDockerImagesTarPath() + heapsterYaml
	//kubessh.SftpSend(sftpClient, yamlFile, destRecvDir)
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, heapsterYamlRecvDir)

	yamlFile = config.GetDockerImagesTarPath() + heapsterRbacYaml
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, heapsterYamlRecvDir)
	//kubessh.SftpSend(sftpClient, yamlFile, destRecvDir)

	url := "http://" + k8sMasterSSH.HostAddr + ":" + config.GetWorkerPort() + "/flash/jobs/exec"

	optCMD := OptCMD{}

	runconf.Read(runconf.DataTypeHeapsterCMDConf, &optCMD)

	c.modifyCMD(optCMD.CreateCMDList, "", "")

	for _, createCMD := range optCMD.CreateCMDList {
		req := msg.Request{
			URL:     url,
			Type:    msg.POST,
			Content: createCMD,
		}

		_, err = req.SendRequestByJSON()
		if err != nil {
			logdebug.Println(logdebug.LevelError, "创建heapster失败:", err.Error())

			return err
		}
	}

	logdebug.Println(logdebug.LevelInfo, "----创建heapster成功----")

	return err
}
