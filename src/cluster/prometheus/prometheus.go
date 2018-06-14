package prometheus

import (
	"fmt"
	//"github.com/pkg/errors"
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

//EMailConf 邮件配置信息(没有单独配置发送者的 都采用全局发送者邮箱)
type EMailConf struct {
	SMTPFrom            string `json:"smtpFrom"`            //全局邮件发送者
	SMTPSmartHost       string `json:"smtpSmartHost"`       //全局邮件服务器
	SMTPAuthUsername    string `json:"smtpAuthUsername"`    //全局发送者邮箱账户
	SMTPAuthPassword    string `json:"smtpAuthPassword"`    //全局发送者邮箱密码
	DefaultReceiver     string `json:"defaultReceiver"`     //默认接收者
	DefaultEmailTo      string `json:"defaultEmailTo"`      //默认接收者邮箱
	DefaultEmailFrom    string `json:"defaultEmailFrom"`    //默认发送者邮箱
	DefaultSmartHost    string `json:"defaultSmartHost"`    //默认邮件服务器
	DefautlAuthUsername string `json:"defautlAuthUsername"` //默认发送者邮箱账户
	DefautlAuthPassword string `json:"defautlAuthPassword"` //默认发送者邮箱密码
}

//Creator prometheus创建器
type Creator struct {
	SessionID    int           `json:"sessionID"`
	Cpu          int           `json:"cpu"`
	Mem          int           `json:"mem"`
	Namespace    string        `json:"namespace"`
	ReplicaCount int           `json:"replicaCount"`
	Timeout      time.Duration `json:"timeout"` //单位秒 (int64)
	EmailCfg     EMailConf     `json:"emailCfg"`
}

//OptCMD 安装操作命令
type OptCMD struct {
	StorageCMDList []cmd.ExecInfo `json:"storageCMDList"`
	DockerCMDList  []cmd.ExecInfo `json:"dockerCMDList"`
	YamlCMDList    []cmd.ExecInfo `json:"yamlCMDList"`
	CreateCMDList  []cmd.ExecInfo `json:"createCMDList"`
}

const (
	prometheusDeploymentYaml   = "/prometheus-deployment.yaml"
	alertmanagerDeploymentYaml = "/alertmanager-deployment.yaml"
	nodeExporterYaml           = "/node-exporter.yaml"
	kubeAPIExporterYaml        = "/kube-api-exporter.yaml"
	prometheusYamlRecvDir      = "/opt/prometheus"
	kubeSystemSecretYaml       = "/kube-system-secret.yaml"
)

func (c *Creator) modifyCMD(src []cmd.ExecInfo, dockerRegistryURL, k8sAPIServerURL, cephMonValue string) {
	dockerImagesPath := config.GetDockerImagesTarPath()
	cpu := strconv.Itoa(c.Cpu)
	mem := strconv.Itoa(c.Mem)

	for k := range src {
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DOCKER_IMAGES_PATH)", dockerImagesPath, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DOCKER_REGISTRY_URL)", dockerRegistryURL, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(CPU)", cpu, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(MEM)", mem, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(K8S_API_URL)", k8sAPIServerURL, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEST_RECV_DIR)", prometheusYamlRecvDir, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(APISERVER_URL_VALUE)", k8sAPIServerURL, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(SMTP_FROM_VALUE)", c.EmailCfg.SMTPFrom, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(SMTP_SMARTHOST_VALUE)", c.EmailCfg.SMTPSmartHost, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(SMTP_AUTH_USERNAME_VALUE)", c.EmailCfg.SMTPAuthUsername, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(SMTP_AUTH_PASSWORD_VALUE)", c.EmailCfg.SMTPAuthPassword, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_RECEIVER_VALUE)", c.EmailCfg.DefaultReceiver, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_EMAIL_TO_VALUE)", c.EmailCfg.DefaultEmailTo, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_EMAIL_FROM_VALUE)", c.EmailCfg.DefaultEmailFrom, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_SMARTHOST_VALUE)", c.EmailCfg.DefaultSmartHost, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_AUTH_USERNAME_VALUE)", c.EmailCfg.DefautlAuthUsername, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(DEFAULT_AUTH_PASSWORD_VALUE)", c.EmailCfg.DefautlAuthPassword, -1)
		src[k].CMDContent = strings.Replace(src[k].CMDContent, "$(CEPH_MONS_VALUE)", cephMonValue, -1)
	}

	return
}

//CheckEmailArgs 检查邮箱配置参数
func (c *Creator) CheckEmailArgs() bool {
	if c.EmailCfg.SMTPAuthPassword == "" {
		return false
	}

	if c.EmailCfg.SMTPAuthUsername == "" {
		return false
	}

	if c.EmailCfg.SMTPSmartHost == "" {
		return false
	}

	if c.EmailCfg.SMTPFrom == "" {
		return false
	}

	if c.EmailCfg.DefaultReceiver == "" {
		return false
	}

	if c.EmailCfg.DefautlAuthPassword == "" {
		return false
	}

	if c.EmailCfg.DefautlAuthUsername == "" {
		return false
	}

	if c.EmailCfg.DefaultSmartHost == "" {
		return false
	}

	if c.EmailCfg.DefaultEmailFrom == "" {
		return false
	}

	if c.EmailCfg.DefaultEmailTo == "" {
		return false
	}

	return true
}

//CreateStorage 创建存储
func (c *Creator) CreateStorage(cephMonNodes []string, cephNodesSSH map[string]kubessh.LoginInfo) error {
	//optCMD := OptCMD{}
	//
	//err := runconf.Read(runconf.DataTypePrometheusCMDConf, &optCMD)
	//if err != nil {
	//	logdebug.Println(logdebug.LevelError, "---读取prometheus命令json文件失败---", err.Error())
	//
	//	return err
	//}
	//
	//if len(cephMonNodes) == 0 {
	//	return errors.New("没有ceph MON信息")
	//}
	//
	//anyMonIP := cephMonNodes[0]
	//monSSH := cephNodesSSH[anyMonIP]

	return nil
}

//PushDockerImage 上传docker镜像
func (c *Creator) PushDockerImage(dockerRegistryURL string, k8sAPIServerURL string) error {
	optCMD := OptCMD{}

	err := runconf.Read(runconf.DataTypePrometheusCMDConf, &optCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---读取prometheus命令json文件失败---", err.Error())

		return err
	}

	c.modifyCMD(optCMD.DockerCMDList, dockerRegistryURL, k8sAPIServerURL, "")
	logdebug.Println(logdebug.LevelInfo, "---读取prometheus命令json文件内容---", optCMD.DockerCMDList)

	//调试时 push太慢 暂时注释
	for _, v := range optCMD.DockerCMDList {
		v.Exec()
	}

	return nil
}

//ModifyYaml 修改yaml文件
func (c *Creator) ModifyYaml(dockerRegistryURL, k8sAPIServerURL string, cephMons []string) error {
	optCMD := OptCMD{}
	cephMonValue := ""

	for _, v := range cephMons {
		cephMon := fmt.Sprintf(`          - '%s:6789'\\n`,
			v,
		)
		cephMonValue += cephMon
	}

	err := runconf.Read(runconf.DataTypePrometheusCMDConf, &optCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---读取prometheus命令json文件失败---", err.Error())

		return err
	}

	c.modifyCMD(optCMD.YamlCMDList, dockerRegistryURL, k8sAPIServerURL, cephMonValue)

	logdebug.Println(logdebug.LevelInfo, "---读取prometheus命令json文件内容---", optCMD.YamlCMDList)

	for _, v := range optCMD.YamlCMDList {
		v.Exec()
	}

	return nil
}

//Build 构建prometheus组件
func (c *Creator) Build(k8sMasterSSH kubessh.LoginInfo) error {
	logdebug.Println(logdebug.LevelInfo, "---构建prometheus---", *c)

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

	mkdirCMD := fmt.Sprintf(`sudo mkdir -p %s`, prometheusYamlRecvDir)

	kubessh.ClientRunCMD(c.SessionID, sshClient, cmd.ExecInfo{CMDContent: mkdirCMD})

	//发送到对端/opt目录
	yamlFile := config.GetDockerImagesTarPath() + prometheusDeploymentYaml
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, prometheusYamlRecvDir)

	yamlFile = config.GetDockerImagesTarPath() + alertmanagerDeploymentYaml
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, prometheusYamlRecvDir)

	yamlFile = config.GetDockerImagesTarPath() + nodeExporterYaml
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, prometheusYamlRecvDir)

	yamlFile = config.GetDockerImagesTarPath() + kubeAPIExporterYaml
	kubessh.ExecSendFileCMD(sshClient, sftpClient, yamlFile, prometheusYamlRecvDir)

	//可能需要sudo权限 将rbd命令加入/etc/sudoers
	//rbd create -p rbd prometheus-rbd --size 128M
	//rbd create -p rbd alertmanager-rbd --size 128M

	url := "http://" + k8sMasterSSH.HostAddr + ":" + config.GetWorkerPort() + "/flash/jobs/exec"

	optCMD := OptCMD{}

	runconf.Read(runconf.DataTypePrometheusCMDConf, &optCMD)

	c.modifyCMD(optCMD.CreateCMDList, "", "", "")

	for _, createCMD := range optCMD.CreateCMDList {
		req := msg.Request{
			URL:     url,
			Type:    msg.POST,
			Content: createCMD,
		}

		logdebug.Println(logdebug.LevelInfo, "----执行命令---", createCMD)

		_, err = req.SendRequestByJSON()
		if err != nil {
			logdebug.Println(logdebug.LevelError, "创建prometheus失败:", err.Error())

			return err
		}
	}

	logdebug.Println(logdebug.LevelInfo, "----创建prometheus成功----")

	return err
}
