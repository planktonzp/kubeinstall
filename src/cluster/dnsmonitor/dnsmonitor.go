package dnsmonitor

import (
	//"fmt"
	//"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/runconf"
	//"kubeinstall/src/msg"
	"strconv"
	"strings"
	"time"
)

//Creator dnsmonitor创建器
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
	dnsMonitorYaml = "/dnsmonitor.yaml"
	//dnsMonitorYamlRecvDir = "/dnsmonitor"
	dnsMonitorYamlOptDir = "/opt/dnsmonitor"
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
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEST_RECV_DIR)", dnsMonitorYamlOptDir, -1)
	}

	return
}

//PushDockerImage 上传docker镜像
func (c *Creator) PushDockerImage(dockerRegistryURL string) error {
	//BCM会自动创建dns-monitor 后端只需要上传镜像即可
	optCMD := OptCMD{}

	err := runconf.Read(runconf.DataTypeDNSMonitorCMDConf, &optCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---读取dns-monitor命令json文件失败---", err.Error())

		return err
	}

	c.modifyCMD(optCMD.DockerCMDList, dockerRegistryURL, "")
	logdebug.Println(logdebug.LevelInfo, "---读取dns-monitor命令json文件内容---", optCMD.DockerCMDList)

	//调试时 push太慢 暂时注释
	for _, v := range optCMD.DockerCMDList {
		v.Exec()
	}

	return nil
}

//ModifyYaml 修改yaml文件
func (c *Creator) ModifyYaml(dockerRegistryURL, k8sAPIServerURL string) error {

	return nil
}

//Build 构建dnsmonitor组件
func (c *Creator) Build(k8sMasterSSH kubessh.LoginInfo) error {
	logdebug.Println(logdebug.LevelInfo, "---构建dnsmonitor---", *c)

	//yamlFile := config.GetDockerImagesTarPath() + dnsMonitorYamlName
	//
	//sshClient, err := k8sMasterSSH.ConnectToHost()
	//if err != nil {
	//	return err
	//}
	//
	//defer sshClient.Close()
	//
	//sftpClient, err := sftp.NewClient(sshClient)
	//if err != nil {
	//	return err
	//}
	//
	//defer sftpClient.Close()
	//
	//destRecvDir := config.GetDestReceiverPath() + dnsMonitorYamlRecvDir
	//
	//mkdirCMD := fmt.Sprintf(`mkdir %s`, destRecvDir)
	//
	//kubessh.ClientRunCMD(c.SessionID, sshClient, cmd.ExecInfo{CMDContent: mkdirCMD})
	//
	////发送到对端/opt目录
	//kubessh.SftpSend(sftpClient, yamlFile, destRecvDir)
	//
	//url := "http://" + k8sMasterSSH.HostAddr + ":" + config.GetWorkerPort() + "/flash/jobs/exec"
	//createCMD := cmd.ExecInfo{
	//	CMDContent: fmt.Sprintf(`kubectl create -f %s`, destRecvDir),
	//	EnvSet:     []string{"KUBECONFIG=/etc/kubernetes/admin.conf"},
	//}
	//
	//req := msg.Request{
	//	URL:     url,
	//	Type:    msg.POST,
	//	Content: createCMD,
	//}
	//
	//_, err = req.SendRequestByJSON()
	//if err != nil {
	//	logdebug.Println(logdebug.LevelError, "创建DNS monitor失败:", err.Error())
	//}

	return nil
}
