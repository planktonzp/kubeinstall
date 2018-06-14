package cluster

import (
	"fmt"
	"github.com/pkg/errors"
	//"github.com/pkg/sftp"
	//"kubeinstall/src/cmd"
	"golang.org/x/crypto/ssh"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/runconf"
	"kubeinstall/src/state"
	"os"
	"strconv"
	"strings"
)

//MasterConf 搭建k8s集群master信息
type MasterConf struct {
	K8sMasterIPSet []string `json:"k8sMasterIPSet"` //用作k8s master IP
	VirtualIP      string   `json:"virtualIP"`
	Port           string   `json:"port"`
	//	NetworkCNI     string                       `json:"networkCNI"`
	PodNetworkCIDR string                       `json:"podNetworkCIDR"`
	AccessIPList   map[string]kubessh.LoginInfo `json:"accessIPList"` //用于外部访问的IP (kubeworker)
	K8sVersion     string                       `json:"k8sVersion"`   //如果用户没有配置 则使用config中配置的
	InterfaceName  string                       `json:"interfaceName"`
}

/*
//不再添加CNI
//CNIFlannel 暂时仅支持这2种
const (
	CNIFlannel = "flannel"
	CNICalico  = "calico"
)
*/

const (
	apiserverIPValue    = "$(API_SERVER_IP_VALUE)"
	kubeRepoPrefixValue = "$(KUBE_REPO_PREFIX_VALUE)"
)

const (
	kubeadmConfigYamlName = "config.yaml"
	//calicoYamlName        = "/calico.yaml"
	//flannelRbacYamlName   = "/kube-flannel-rbac.yaml"
	//flannelYamlName       = "/kube-flannel.yaml"
)

const (
	keepalivedStateMaster = "MASTER"
	keepalivedStateBackup = "BACKUP"
)

//master LB相关
const (
	scriptName     = "chk_haproxy"
	scriptSuffix   = ".sh"
	keepaliveState = "MASTER"
	//interfaceName         = "eth0"
	priority = 100
	weight   = 6
	//keepalivedCfgSrcPath  = "/tmp/keepalived.conf"
	keepalivedCfgDestPath = "/etc/keepalived/"
	//haproxyCfgSrcPath     = "/tmp/haproxy.cfg"
	haproxyCfgDestPath = "/etc/haproxy/"
)

//var sessionID int

func (p *InstallPlan) checkMaster() msg.ErrorCode {
	masterCount := len(p.MasterCfg.K8sMasterIPSet)
	if masterCount == 0 {
		return msg.MasterNotSpecified
	}

	if masterCount == 1 {
		return msg.MasterNotHighAvailable
	}

	if p.MasterCfg.VirtualIP == "" {
		return msg.HAVirtualIPNotSpecified
	}

	/*
		//不再检查cni组件
		if p.MasterCfg.NetworkCNI == "" {
			return msg.NetworkCNINotSpecified
		}
	*/

	return msg.Success
}

/* 原始模板
apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: <address|string>
  bindPort: <int>
etcd:
  endpoints:
  - <endpoint1|string>
  - <endpoint2|string>
  caFile: <path|string>
  certFile: <path|string>
  keyFile: <path|string>
networking:
  dnsDomain: <string>
  serviceSubnet: <cidr>
  podSubnet: <cidr>
kubernetesVersion: <string>
cloudProvider: <string>
authorizationModes:
- <authorizationMode1|string>
- <authorizationMode2|string>
token: <string>
tokenTTL: <time duration>
selfHosted: <bool>
apiServerExtraArgs:
  <argument>: <value|string>
  <argument>: <value|string>
controllerManagerExtraArgs:
  <argument>: <value|string>
  <argument>: <value|string>
schedulerExtraArgs:
  <argument>: <value|string>
  <argument>: <value|string>
apiServerCertSANs:
  - <name1|string>
  - <name2|string>
certificatesDir: <string>
*/
/* 目标
apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: 192.168.0.199
etcd:
  endpoints:
  - http://192.168.0.77:2379
  - http://192.168.0.78:2379
  - http://192.168.0.79:2379
networking:
  podSubnet: 10.244.0.0/16
kubernetesVersion: v1.6.3
*/
func (p *InstallPlan) createConfigYaml() (string, error) {
	var (
		etcdCfg    string
		networkCfg string
	)

	configYamlFilePath := config.GetYamlFilePath()

	yaml := configYamlFilePath + kubeadmConfigYamlName

	file, err := os.Create(yaml)
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return "", err
	}

	defer file.Close()

	//if p.MasterCfg.NetworkCNI == CNIFlannel {
	//修改config.yaml
	networkCfg = fmt.Sprintf(`networking:
  podSubnet: %s`,
		p.MasterCfg.PodNetworkCIDR,
	)
	//}

	if len(currentClusterStatus.Etcd) == defaultEtcdClusterNodeCounts {
		//修改config.yaml
		etcdCfg = fmt.Sprintf(`etcd:
  endpoints:
  - http://%s:2379
  - http://%s:2379
  - http://%s:2379`,
			currentClusterStatus.Etcd[0],
			currentClusterStatus.Etcd[1],
			currentClusterStatus.Etcd[2],
		)
	}

	content := fmt.Sprintf(`apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: %s
  bindPort: %s
%s
%s
kubernetesVersion: %s
imageRepository: %s

apiServerExtraArgs:
  #runtime-config: "api/all=true"
  #feature-gates: "AdvancedAuditing=true"
  #v: "8"
  runtime-config: "batch/v2alpha1=true"
  audit-log-path: "/etc/pki/audit.log"
  audit-log-maxage: "1"
  audit-log-maxbackup: "1"
  audit-log-maxsize: "10"
  service-node-port-range: "%s"
  #audit-policy-file: "/etc/kubernetes/audit-policy.yaml"

`,
		p.MasterCfg.VirtualIP,
		p.MasterCfg.Port,
		etcdCfg,
		networkCfg,
		p.MasterCfg.K8sVersion,
		currentClusterStatus.DockerRegistryURL,
		config.GetSvcPortRange(),
	)

	file.WriteString(content)

	return yaml, nil
}

/*
//不再安装CNI
func installCalicoCNI(sessionID int, client *ssh.Client, fastSetupURL string, cmdList []cmd.ExecInfo) {
	yamlFile := config.GetDockerImagesTarPath() + calicoYamlName

	err := sshSendFile(client, yamlFile, receiverOPTPath)
	if err != nil {
		return
	}

	cmdList = addSchedulePercent(cmdList, "30%")
	sendCMDToK8scc(sessionID, fastSetupURL, cmdList)

	return
}

//不再安装CNI
func installFlannelCNI(sessionID int, client *ssh.Client, fastSetupURL string, cmdList []cmd.ExecInfo) {
	yamlFile := config.GetDockerImagesTarPath() + flannelRbacYamlName

	err := sshSendFile(client, yamlFile, receiverOPTPath)
	if err != nil {
		return
	}

	yamlFile = config.GetDockerImagesTarPath() + flannelYamlName

	err = sshSendFile(client, yamlFile, receiverOPTPath)
	if err != nil {
		return
	}
	cmdList = addSchedulePercent(cmdList, "30%")
	sendCMDToK8scc(sessionID, fastSetupURL, cmdList)

	return
}
*/

/*
//不再安装CNI 直接初始化空集群(无网络)
//暂时不做结果处理
func (p *InstallPlan) installNetworkCNI(sessionID int, client *ssh.Client, fastSetupURL string, conf kubeWorkerCMDConf) {
	//将修改后的网络CNI相关yaml发送给master 并通过yaml文件创建CNI
	//cmd 5
	p.MasterCfg.NetworkCNI = config.GetCNIType()

	if p.MasterCfg.NetworkCNI == CNICalico {
		installCalicoCNI(sessionID, client, fastSetupURL, conf.InstallCalicoCMDList)

		return
	}

	//cdm 5
	if p.MasterCfg.NetworkCNI == CNIFlannel {
		installFlannelCNI(sessionID, client, fastSetupURL, conf.InstallFlannelCMDList)

		return
	}

	//others CNI ...
	return
}
*/

func exportAdminConfEnv(sessionID int, fastSetupURL string, conf kubeWorkerCMDConf) {

	output, _ := sendCMDToK8scc(sessionID, fastSetupURL, []cmd.ExecInfo{conf.CheckEtcProfileCMD})
	if output != "" {
		logdebug.Println(logdebug.LevelInfo, "------/etc/profile admin.conf参数曾经被修改过-----")

		return
	}

	logdebug.Println(logdebug.LevelInfo, "------修改/etc/profile 增加admin.conf环境变量-----")

	sendCMDToK8scc(sessionID, fastSetupURL, conf.ModifyEtcProfileCMDList)

	return
}

func (p *InstallPlan) sendMasterLBCfgToHost(client *ssh.Client) {
	vip := p.MasterCfg.VirtualIP

	accessIP := p.MasterCfg.AccessIPList[p.MasterCfg.K8sMasterIPSet[0]]

	sshClient, err := accessIP.ConnectToHost()
	if err != nil {
		logdebug.Println(logdebug.LevelError, "master连接出错", err.Error())

		return
	}

	defer sshClient.Close()

	//如果界面没有配置网卡 则使用配置文件中的网卡
	interfaceName := p.MasterCfg.InterfaceName
	if interfaceName == "" {
		interfaceName = config.GetMasterInterfaceName()
	}

	keepalivedConfFile, err := p.newKeepalivedCfg(keepaliveState, interfaceName, vip, weight, priority)
	//keepalivedConfFile, err := createKeepalivedCfg(vip)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "--生成keepalive配置文件出错---", err.Error())

		return
	}

	err = sshSendFile(sshClient, keepalivedConfFile, keepalivedCfgDestPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "--Send keepalive配置文件出错---", err.Error())

		return
	}

	haproxyConfFile, err := p.newHaproxyCfg()
	//haproxyConfFile, err := createHaproxyCfg(p.MasterCfg.K8sMasterIPSet[0], p.MasterCfg.Port)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "--生成haproyx配置文件出错---", err.Error())

		return
	}

	err = sshSendFile(sshClient, haproxyConfFile, haproxyCfgDestPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "--Send keepalive配置文件出错---", err.Error())

		return
	}

	return
}

func dynamicModifyInitStandaloneMasterCMDList(kubeRepoPrefix string, oldCMDList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}
	newEnvSet := []string{}

	for _, cmdInfo := range oldCMDList {
		for _, env := range cmdInfo.EnvSet {
			env = strings.Replace(env, kubeRepoPrefixValue, kubeRepoPrefix, -1)

			newEnvSet = append(newEnvSet, env)
		}

		cmdInfo.EnvSet = newEnvSet
		//查找到内容 则替换 查找不到 也应存放至新的list中
		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func getK8sToken(sessionID int, masterK8sccURL string, getK8sTokenCMD cmd.ExecInfo) string {
	respBody, err := sendCMDToK8scc(sessionID, masterK8sccURL, []cmd.ExecInfo{getK8sTokenCMD})
	if err != nil {
		return ""
	}

	body := strings.Split(respBody, "\n")
	k8sToken := body[0]

	//重新生成一遍token
	if k8sToken == "" {
		createK8sTokenCMD := cmd.ExecInfo{
			CMDContent: "kubeadm token create --groups system:bootstrappers:kubeadm:default-node-token",
		}
		sendCMDToK8scc(sessionID, masterK8sccURL, []cmd.ExecInfo{createK8sTokenCMD})
	}

	respBody, err = sendCMDToK8scc(sessionID, masterK8sccURL, []cmd.ExecInfo{getK8sTokenCMD})
	if err != nil {
		return ""
	}

	body = strings.Split(respBody, "\n")
	k8sToken = body[0]

	return k8sToken
}

//创建单节点master
//"kubeadm init --config /opt/config.yaml
func (p *InstallPlan) createStandaloneMaster(sessionID int) error {
	var (
		conf    kubeWorkerCMDConf
		context state.Context
		//nodeState node.NodeContext

	)

	context.State = state.Error
	//获取访问IP 以便得到是与哪个fastSetup通讯
	k8sIP := p.MasterCfg.K8sMasterIPSet[0]
	accessIP, ok := p.MasterCfg.AccessIPList[k8sIP]

	newNodeScheduler(Step6, accessIP.HostAddr, "%0", "正在创建单节点master!")

	if !ok {
		errMsg := fmt.Sprintf(`IP:%s 没有提供访问的ip 无法操作`, k8sIP)
		context.ErrMsg = errMsg
		state.Update(sessionID, context)

		setErrNodeScheduler(accessIP.HostAddr, "", errMsg, "安装单节点master失败")

		return errors.New(errMsg)
	}

	err := runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)
	if err != nil {
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(accessIP.HostAddr, "", err.Error(), "安装单节点master失败")

		return err
	}

	//根据已经安装的etcd生成config.yaml 用于kubeadm init --config
	configYamlFile, err := p.createConfigYaml()
	if err != nil {
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(accessIP.HostAddr, "", err.Error(), "安装单节点master失败")

		return err
	}

	sshClient, err := accessIP.ConnectToHost()
	if err != nil {
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(accessIP.HostAddr, "", err.Error(), "安装单节点master失败")

		return err
	}

	defer sshClient.Close()

	//把master的负载均衡相关配置文件发送给目标主机
	p.sendMasterLBCfgToHost(sshClient)

	//send /root/kubeinstall/config.yaml to /opt/
	//发送生成的配置文件
	err = sshSendFile(sshClient, configYamlFile, receiverOPTPath)
	if err != nil {
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(accessIP.HostAddr, "", err.Error(), "安装单节点master失败")

		return err
	}

	fastSetupURL := "http://" + accessIP.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	//TODO:从json中获取之后 改为动态值 目前写死了 k8sIP对应的主机 命令环境变量
	kubeRepoPrefix := currentClusterStatus.DockerRegistryURL
	newCMDList := dynamicModifyInitStandaloneMasterCMDList(kubeRepoPrefix, conf.InitStandaloneMasterCMDList)
	logdebug.Println(logdebug.LevelInfo, "--------修改后的init standalone master newCMDList-------", newCMDList)

	//发送执行kubadm init的命令
	//cmd 1
	newCMDList = addSchedulePercent(newCMDList, "10%")

	updateNodeScheduler(accessIP.HostAddr, "10%", state.Running, "正在初始化master")

	_, err = sendCMDToK8scc(sessionID, fastSetupURL, newCMDList)
	//if err != nil {
	//	return output, err
	//}
	//安装失败也要往下走 防止集群本身可用 却因预检失败 而导致的集群已经创建 却无法创建网络的场景

	//发送获取token的命令
	//cmd 2
	conf.GetK8sTokenCMD.SchedulePercent = "20%"

	updateNodeScheduler(accessIP.HostAddr, "20%", state.Running, "正在获取token")

	k8sToken := getK8sToken(sessionID, fastSetupURL, conf.GetK8sTokenCMD)

	logdebug.Println(logdebug.LevelInfo, "--------k8sToken-------", k8sToken)
	if k8sToken == "" {
		logdebug.Println(logdebug.LevelError, "token获取失败")

		return errors.New("token获取失败")
	}

	//添加环境变量 kubectl命令要使用crt 在json文件中动态订制

	if len(p.MasterCfg.K8sMasterIPSet) > 1 {
		updateNodeScheduler(accessIP.HostAddr, "30%", state.Running, "正在添加环境变量")
	} else {
		updateNodeScheduler(accessIP.HostAddr, "50%", state.Running, "正在添加环境变量")
	}

	exportAdminConfEnv(sessionID, fastSetupURL, conf)

	//安装网络- 认为不会出错
	//添加环境变量 kubectl命令要使用crt 在json文件中动态订制

	//p.installNetworkCNI(sessionID, sshClient, fastSetupURL, conf)
	if len(p.MasterCfg.K8sMasterIPSet) > 1 {
		updateNodeScheduler(accessIP.HostAddr, "40%", state.Running, "正在安装网络插件")
	} else {
		updateNodeScheduler(accessIP.HostAddr, "100%", state.Running, "正在安装网络插件")
	}

	if len(p.MasterCfg.K8sMasterIPSet) > 1 {
		updateNodeScheduler(accessIP.HostAddr, "", state.Running, "完成安装单个主节点")
	} else {
		updateNodeScheduler(accessIP.HostAddr, "100%", state.Complete, "完成安装主节点")
	}

	//手工执行nohup kubectl proxy --address=0.0.0.0 --accept-hosts=^*$ -p 23333 &
	//kubeadm token create --groups system:bootstrappers:kubeadm:default-node-token
	//保存 token 等集群信息 k8s版本 端口等信息以后将可以通过用户端配置 现在是通过kubeinstall配置
	currentClusterStatus.Token = k8sToken //token 不做特殊配置 24小时内删除 可以重建并关联
	currentClusterStatus.Masters = p.MasterCfg.K8sMasterIPSet
	currentClusterStatus.APIServerURL = p.MasterCfg.VirtualIP + ":" + p.MasterCfg.Port
	currentClusterStatus.ProxyAPIServerURL = p.MasterCfg.VirtualIP + ":" + strconv.Itoa(config.GetK8sProxyPort())
	currentClusterStatus.MasterVirtualIP = p.MasterCfg.VirtualIP
	currentClusterStatus.HealthState = stateHealthy
	currentClusterStatus.Version = p.MasterCfg.K8sVersion
	currentClusterStatus.InstallTime = getNowUTCTime()

	//保存所有master SSH信息
	for _, v := range p.MasterCfg.K8sMasterIPSet {
		//先拿到ssh信息
		masterSSH := p.MachineSSHSet[v]
		//更新到当前map
		currentClusterStatus.NodesSSH[v] = masterSSH
	}

	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return err
}

func (p *InstallPlan) newKeepalivedCfg(state, interfaceName, vip string, weight, priority int) (string, error) {
	virtualRouterID := config.GetVirtualRouterID()
	keepalivedCfg := fmt.Sprintf(`
! Configuration File for keepalived

global_defs {
    notification_email {
     root@localhost
   }
   notification_email_from root@localhost
   smtp_server localhost
   smtp_connect_timeout 30
   router_id  NodeA
   vrrp_skip_check_adv_addr
   #vrrp_strict
   vrrp_garp_interval 0
   vrrp_gna_interval 0
}

vrrp_script chk_haproxy {
 #script  "/etc/keepalived/chk_haproxy.sh"
 script  "/etc/keepalived/%s"
 interval 5 # check every 5 seconds
 weight %d
 #fall 2 # require 2 fail for KO
 #rise 1 # require 1 successes for OK
}

vrrp_instance VI_1 {
    state %s
    #state MASTER
    interface %s
    #interface eth0
    virtual_router_id %s
    priority %d
    #priority 100
    advert_int 1
    authentication {
        auth_type PASS
        auth_pass 1111
    }
    virtual_ipaddress {
        %s
        #192.168.0.199
    }
    track_script {
        %s
        #chk_haproxy
    }
}

`,
		scriptName+scriptSuffix,
		weight,
		state,
		interfaceName,
		virtualRouterID,
		priority,
		vip,
		scriptName,
	)

	//远端文件地址/etc/keepalived/keepalived.conf
	//cfgPath := keepalivedCfgSrcPath
	//cfgPath := config.GetWorkDir() + "/kubeinstall/keepalived.conf"
	cfgPath := config.GetDownloadsDir() + "keepalived.conf"

	file, err := os.Create(cfgPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return "", err
	}

	defer file.Close()

	file.WriteString(keepalivedCfg)

	return cfgPath, nil
}

func (p *InstallPlan) newHaproxyCfg() (string, error) {
	allServer := ""

	//有几个master就会生成几条server
	for i, ip := range p.MasterCfg.K8sMasterIPSet {
		serverCfg := fmt.Sprintf(`server s%d %s:%s weight 1 maxconn 10000 check inter 10s
`,
			i+1, //serverNumber
			ip,
			p.MasterCfg.Port,
		)
		allServer += serverCfg
	}

	haproxyCfg := fmt.Sprintf(`
########默认配置############
defaults
mode tcp               #默认的模式mode { tcp|http|health }，tcp是4层，http是7层，health只会返回OK
retries 3              #两次连接失败就认为是服务器不可用，也可以通过后面设置
option redispatch      #当serverId对应的服务器挂掉后，强制定向到其他健康的服务器
option abortonclose    #当服务器负载很高的时候，自动结束掉当前队列处理比较久的链接
maxconn 32000          #默认的最大连接数
timeout connect 5000ms #连接超时
timeout client 30000ms #客户端超时
timeout server 30000ms #服务器超时
#timeout check 2000    #心跳检测超时
log 127.0.0.1 local0 err #[err warning info debug]

########test1配置#################
listen testkubeapiserver
bind 0.0.0.0:9908
mode tcp
balance roundrobin
%s

########统计页面配置########
listen admin_stats
bind 0.0.0.0:8099 #监听端口
mode http         #http的7层模式
option httplog    #采用http日志格式
#log 127.0.0.1 local0 err
maxconn 10
stats enable
stats refresh 30s #统计页面自动刷新时间
stats uri /stats #统计页面url
#stats realm XingCloud\ Haproxy #统计页面密码框上提示文本
stats auth admin:admin #统计页面用户名和密码设置
stats admin if TRUE
#stats hide-version #隐藏统计页面上HAProxy的版本信息

`,
		allServer,
	)

	//远端文件地址/etc/haproxy/haproxy.cfg
	//cfgPath := haproxyCfgSrcPath
	//cfgPath := config.GetWorkDir() + "/kubeinstall/haproxy.cfg"
	cfgPath := config.GetDownloadsDir() + "haproxy.cfg"

	file, err := os.Create(cfgPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, err)

		return "", err
	}

	defer file.Close()

	file.WriteString(haproxyCfg)

	return cfgPath, nil
}

func restartLoadBalance(sessionID int, ip string) error {
	var conf kubeWorkerCMDConf

	url := "http://" + ip + ":" + config.GetWorkerPort() + workerServerPath

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//cmd 6
	conf.RestartLoadBalanceCMDList = addSchedulePercent(conf.RestartLoadBalanceCMDList, "40%")
	_, err := sendCMDToK8scc(sessionID, url, conf.RestartLoadBalanceCMDList)

	return err
}

func (p *InstallPlan) modifyLoadBalance(sessionID int, innerIP, state string, priority int) error {
	//masterIP := p.MasterCfg.K8sMasterIPSet[0]
	hostSSH := p.MasterCfg.AccessIPList[innerIP]

	//如果界面没有配置网卡 则使用配置文件中的网卡
	interfaceName := p.MasterCfg.InterfaceName
	if interfaceName == "" {
		interfaceName = config.GetMasterInterfaceName()
	}

	keepalivedCfg, err := p.newKeepalivedCfg(state, interfaceName, p.MasterCfg.VirtualIP, weight, priority)
	if err != nil {
		return err
	}

	haproxyCfg, err := p.newHaproxyCfg()
	if err != nil {
		return err
	}

	client, err := hostSSH.ConnectToHost()
	if err != nil {
		return err
	}

	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		//fmt.Println("-----newClient----", err)
		return err
	}

	defer sftpClient.Close()

	//将修改后的配置发送给对应的keepailved配置目录以及haproxy配置目录
	//kubessh.SftpSend(sftpClient, keepalivedCfg, keepalivedCfgDestPath)
	kubessh.ExecSendFileCMD(client, sftpClient, keepalivedCfg, keepalivedCfgDestPath)
	//kubessh.SftpSend(sftpClient, haproxyCfg, haproxyCfgDestPath)
	kubessh.ExecSendFileCMD(client, sftpClient, haproxyCfg, haproxyCfgDestPath)

	return restartLoadBalance(sessionID, hostSSH.HostAddr)
}

func (p *InstallPlan) modifyBackupLoadBalance(sessionID int) error {
	var (
		err error
	)

	bkpPriority := priority - weight + 1
	for i, innerIP := range p.MasterCfg.K8sMasterIPSet {
		if i == 0 {
			//首个master已经配置完成
			continue
		}

		newNodeScheduler(Step6, p.MasterCfg.AccessIPList[innerIP].HostAddr, "30%", "正在配置副节点")

		err = p.modifyLoadBalance(sessionID, innerIP, keepalivedStateBackup, bkpPriority)
		if err != nil {
			return err
		}
	}

	return err
}

func (p *InstallPlan) bkpMastersJoin(sessionID int) error {
	var (
		err error
	)

	var conf kubeWorkerCMDConf

	err = runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)
	if err != nil {
		return err
	}

	conf.NodeJoinCMDList = dynamicModifyNodeJoinCMD(conf.NodeJoinCMDList)
	conf.NodeJoinCMDList = addSchedulePercent(conf.NodeJoinCMDList, "60%")
	logdebug.Println(logdebug.LevelInfo, "-----修改后的cmd--------", conf.NodeJoinCMDList)

	for i, innerIP := range p.MasterCfg.K8sMasterIPSet {
		if i == 0 {
			//首个master已经配置完成
			continue
		}

		updateNodeScheduler(p.MasterCfg.AccessIPList[innerIP].HostAddr, "60%", state.Running, "副节点正在加入集群")

		//组织URL 获取命令集合
		hostSSH := p.MasterCfg.AccessIPList[innerIP]
		url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		//发送nodeJoin的命令

		//cmd 7
		_, err = sendCMDToK8scc(sessionID, url, conf.NodeJoinCMDList)
		if err != nil {
			return err
		}

		//添加环境变量 kubectl命令要使用crt 在json文件中动态订制
		exportAdminConfEnv(sessionID, url, conf)

		updateNodeScheduler(p.MasterCfg.AccessIPList[innerIP].HostAddr, "90%", state.Running, "副节点正在配置环境变量")
	}

	return err
}

func (p *InstallPlan) downloadK8sCfg(sessionID int) (string, error) {
	masterInnerIP := p.MasterCfg.K8sMasterIPSet[0]
	masterSSH := p.MasterCfg.AccessIPList[masterInnerIP]

	cmdInfo := cmd.ExecInfo{
		CMDContent: "tar -czPvf /tmp/k8s.tar /etc/kubernetes/",
		//EnvSet:     []string{"USER=root"},
	}

	client, err := masterSSH.ConnectToHost()
	if err != nil {
		return err.Error(), err
	}
	defer client.Close()

	//kubessh.ClientRunCMD(client, cmdInfo)

	url := "http://" + masterSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	output, _ := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
	logdebug.Println(logdebug.LevelInfo, "-----output000----", output)

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err.Error(), err
	}

	defer sftpClient.Close()

	recvDir := config.GetDownloadsDir()

	kubessh.SftpDownload(sftpClient, "/tmp/k8s.tar", recvDir)

	return "在master上打包k8s配置下载到本机", nil
}

func sendK8sCfgToBkpMaster(sessionID int, bkpMasterSSH kubessh.LoginInfo) (string, error) {
	client, err := bkpMasterSSH.ConnectToHost()
	if err != nil {
		return err.Error(), err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err.Error(), err
	}

	defer sftpClient.Close()

	//kubessh.SftpSend(sftpClient, "/home/k8s.tar", "/tmp/")
	kubessh.ExecSendFileCMD(client, sftpClient, config.GetDownloadsDir()+"k8s.tar", "/tmp/")

	cmdInfo := cmd.ExecInfo{
		CMDContent: "tar -zxPvf /tmp/k8s.tar",
	}

	//cmd 8
	cmdInfo.SchedulePercent = "80%"
	//kubessh.ClientRunCMD(sessionID, client, cmdInfo)
	url := "http://" + bkpMasterSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})

	return "", nil
}

func (p *InstallPlan) sendK8sCfg(sessionID int) (string, error) {
	for i, ip := range p.MasterCfg.K8sMasterIPSet {
		if i == 0 {
			//要操作剩余的master
			continue
		}
		bkpMasterSSH := p.MasterCfg.AccessIPList[ip]

		updateNodeScheduler(p.MasterCfg.AccessIPList[ip].HostAddr, "100%", state.Running, "k8s配置发送完成")

		sendK8sCfgToBkpMaster(sessionID, bkpMasterSSH)
	}

	return "发送k8s配置至各个bkp节点", nil
}

func (p *InstallPlan) copyK8sCfgToBkpMaster(sessionID int) error {
	//在master上打包下载到本机
	output, err := p.downloadK8sCfg(sessionID)
	if err != nil {
		return err
	}
	finalOutput := output

	output, err = p.sendK8sCfg(sessionID)
	finalOutput += output
	//然后sftp拷贝到backup主机并解压

	return err
}

//集中修改 原有的单节点master、新增的master主机的keepalived + haproxy配置文件
//在指定的node(新master2 master3)上创建apiserver等k8s组件
//用户需要保证将要安装高可用master的若干主机已经安装了核心的RPM
func (p *InstallPlan) createHAMaster(sessionID int) error {
	//var finalOutput string

	//0.先构建出单master
	//将单master对应的负载均衡器的cfg修改为多节点
	//cmd 6 master1 50%开始 master2 0%开始

	accessIP := p.MasterCfg.AccessIPList[p.MasterCfg.K8sMasterIPSet[0]]

	updateNodeScheduler(accessIP.HostAddr, "50%", state.Running, "正在配置master高可用")

	err := p.modifyLoadBalance(sessionID, p.MasterCfg.K8sMasterIPSet[0], keepalivedStateMaster, priority)
	if err != nil {
		return err
	}

	//cmd 6

	err = p.modifyBackupLoadBalance(sessionID)
	if err != nil {
		return err
	}

	err = p.bkpMastersJoin(sessionID)
	if err != nil {
		return err
	}

	err = p.copyK8sCfgToBkpMaster(sessionID)
	if err != nil {
		return err
	}

	updateNodeScheduler(accessIP.HostAddr, "100%", state.Complete, "完成master高可用")

	//1.将新的节点join进集群，如果已经在集群了应该cordon  && drain
	//2.修改所有master节点的haproxy keepalived配置文件 调整权重 角色 server等参数
	//3.master的STATE不变，新加入的master改为BACKUP
	//4.master的pri - weight要小于backup的pri
	//5.haproxy中添加server
	//6.cp -rf /etc/kubernetes/* 给其他master
	//7.添加ENV KUBECONFIG
	//cd /etc/kubernetes/ && tar -czvf /tmp/test/k8s.tar *

	return err
}

//InitMaster kubeadm init
func (p *InstallPlan) InitMaster(sessionID int) error {
	var (
		errMsg  string
		context state.Context
	)

	context.State = state.Error

	//再检查一遍 防止单独调用接口时提供的参数非法
	errCode := p.checkMaster()
	//允许单节点master
	if errCode != msg.Success && errCode != msg.MasterNotHighAvailable {
		errMsg = msg.GetErrMsg(errCode)
		context.ErrMsg = errMsg
		state.Update(sessionID, context)

		return errors.New(errMsg)
	}

	/*
		//不再使用kubeinstall安装网络组件
		if p.MasterCfg.NetworkCNI == CNIFlannel && p.MasterCfg.PodNetworkCIDR == "" {
			errMsg = "flannel网络需要指定pod-network-cidr参数"
			context.ErrMsg = errMsg
			state.Update(sessionID, context)

			return errors.New(errMsg)
		}
	*/

	if p.MasterCfg.K8sVersion == "" {
		p.MasterCfg.K8sVersion = config.GetK8sVersion()
	}

	if p.MasterCfg.Port == "" {
		p.MasterCfg.Port = config.GetK8sAPIServerPort()
	}

	//0.先构建出单master
	err := p.createStandaloneMaster(sessionID)
	if err != nil {
		return err
	}

	//方便测试 先不创建master
	if len(p.MasterCfg.K8sMasterIPSet) > 1 {
		err = p.createHAMaster(sessionID)
	}

	if err != nil {
		return err
	}

	//var context state.Context
	context.State = state.Complete
	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	return err
}

//RemoveMaster 移除已经安装的master 在machineSSH中提供3台master主机的SSH信息
func (p *InstallPlan) RemoveMaster(moduleSessionID int) (string, error) {
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	multiExecCMD(moduleSessionID, p.MachineSSHSet, conf.ResetMasterCMDList)

	context := state.Context{
		State: state.Complete,
	}

	state.Update(moduleSessionID, context)

	return "移除已经安装的master", nil
}
