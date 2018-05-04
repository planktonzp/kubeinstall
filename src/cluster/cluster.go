package cluster

//YUM相关命令存在阻塞 卡住等情况
//由于yum命令不支持并发 所以测试过程中 不要轻易打断上一次因为yum命令没有执行完而导致的kubeinstall时间过长

import (
	//"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	//"sync"
	//"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/errors"
	//"errors"
	//"entry/logdebug"
	//"kubeinstall/src/backup"
	//"fmt"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	//"kubeinstall/src/client"
	"kubeinstall/src/cmd"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/msg"
	"kubeinstall/src/state"
	//"time"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/config"
	"kubeinstall/src/node"
	"kubeinstall/src/runconf"
	"kubeinstall/src/session"
	//"strings"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"strings"
	"sync"
	"time"
)

//DNS-MASTER ETCD
//拆分 手动/自动

//InstallPlan 统一的参数结构
type InstallPlan struct {
	MachineSSHSet map[string]kubessh.LoginInfo `json:"machineSSHSet"`
	RpmsCfg       RpmsConf                     `json:"rpmsCfg"`
	EtcdCfg       EtcdConf                     `json:"etcdCfg"`
	MasterCfg     MasterConf                   `json:"masterCfg"`
	DockerCfg     DockerConf                   `json:"dockerCfg"`
	YUMCfg        YUMConf                      `json:"yumCfg"`
	NodesCfg      NodesConf                    `json:"nodesCfg"`
	CephCfg       CephConf                     `json:"cephCfg"`
	K8sModCfg     K8sModConf                   `json:"k8sModCfg"`
	TestCfg       TestConf                     `json:"testCfg"`
}

//TestConf 测试配置
type TestConf struct {
	ClusterStatus Status `json:"clusterStatus"`
}

//KubeWorker 与 worker通讯的必要信息
type KubeWorker struct {
	HostAddr string
	Port     string
}

const (
	workerServerPath = "/flash/jobs/exec"
	stateUnhealthy   = "unhealthy"
	stateHealthy     = "healthy"
	receiverOPTPath  = "/opt"
)

//kubeworker相关命令
type kubeWorkerCMDConf struct {
	InstallCoreCMDList          []cmd.ExecInfo `json:"installCoreCMDList"`
	CreateDockerRegistryCMDList []cmd.ExecInfo `json:"createDockerRegistryCMDList"`
	ApplyDockerRegistryCMDList  []cmd.ExecInfo `json:"applyDockerRegistryCMDList"`
	InstallEtcdCMDList          []cmd.ExecInfo `json:"installEtcdCMDList"`
	InitStandaloneMasterCMDList []cmd.ExecInfo `json:"initStandaloneMasterCMDList"`
	InitClusterMasterCMDList    []cmd.ExecInfo `json:"initClusterMasterCMDList"`
	RestartLoadBalanceCMDList   []cmd.ExecInfo `json:"restartLoadBalanceCMDList"`
	GetK8sTokenCMD              cmd.ExecInfo   `json:"getK8sTokenCMD"` //kubeadm token list | grep signing | awk {'print $1'}
	InstallCalicoCMDList        []cmd.ExecInfo `json:"installCalicoCMDList"`
	InstallFlannelCMDList       []cmd.ExecInfo `json:"installFlannelCMDList"`
	ModifyKubeletCMDList        []cmd.ExecInfo `json:"modifyKubeletCMDList"`
	CheckKubeletConfCMD         cmd.ExecInfo   `json:"checkKubeletConfCMD"`
	ModifyEtcProfileCMDList     []cmd.ExecInfo `json:"modifyEtcProfileCMDList"`
	CheckEtcProfileCMD          cmd.ExecInfo   `json:"checkEtcProfileCMD"`
	NodeJoinCMDList             []cmd.ExecInfo `json:"nodeJoinCMDList"`
	CreateDockerStorageCMDList  []cmd.ExecInfo `json:"createDockerStorageCMDList"`
	InstallCephCMDList          []cmd.ExecInfo `json:"installCephCMDList"`
	CreateSSHKeyCMDList         []cmd.ExecInfo `json:"createSSHKeyCMDList"`
	SetMaxProcCMDList           []cmd.ExecInfo `json:"setMaxProcCMDList"`
	SetCephGlobalCfgCMDList     []cmd.ExecInfo `json:"setCephGlobalCfgCMDList"`
	InitMonNodesCMDList         []cmd.ExecInfo `json:"initMonNodesCMDList"`
	AddMonNodesCMDList          []cmd.ExecInfo `json:"addMonNodesCMDList"`
	SetCephClientCMDList        []cmd.ExecInfo `json:"setCephClientCMDList"`
	InstallK8sModulesCMDList    []cmd.ExecInfo `json:"installK8sModulesCMDList"`

	//反操作命令集合
	CleanDockerRegistryCMDList []cmd.ExecInfo `json:"cleanDockerRegistryCMDList"`
	CleanDockerClientCMDList   []cmd.ExecInfo `json:"cleanDockerClientCMDList"`
	CleanEtcdCMDList           []cmd.ExecInfo `json:"cleanEtcdCMDList"`
	ResetMasterCMDList         []cmd.ExecInfo `json:"resetMasterCMDList"`
	DeleteNodesCMDList         []cmd.ExecInfo `json:"deleteNodesCMDList"`
	CleanNodesCMDList          []cmd.ExecInfo `json:"cleanNodesCMDList"`
	RemoveCephCMDList          []cmd.ExecInfo `json:"removeCephCMDList"`
}

//Status 集群状态信息
type Status struct {
	Token                   string                       `json:"token"`
	Masters                 []string                     `json:"masters"`
	Nodes                   []string                     `json:"nodes"`
	Etcd                    []string                     `json:"etcd"`
	InstallTime             string                       `json:"installTime"`
	Version                 string                       `json:"version"`
	HealthState             string                       `json:"healthState"`
	DockerRegistryURL       string                       `json:"dockerRegistryURL"`
	IsDefaultDockerRegistry bool                         `json:"isDefaultDockerRegistry"`
	DockerRegistryHostAddr  string                       `json:"dockerRegistryHostAddr"`
	YUMStorageRepo          repoInfo                     `json:"yumStorageRepo"`
	YUMHostAddr             string                       `json:"yumHostAddr"`
	IsDefaultYUMStorage     bool                         `json:"isDefaultYUMStorage"`
	APIServerURL            string                       `json:"apiServerURL"`
	ProxyAPIServerURL       string                       `json:"proxyAPIServerURL"`
	MasterVirtualIP         string                       `json:"masterVirtualIP"`
	NodesSSH                map[string]kubessh.LoginInfo `json:"nodesSSH"`
	CephMonNodes            []string                     `json:"cephMonNodes"`
	CephOsdNodes            []string                     `json:"cephOsdNodes"`
	CephMdsNodes            []string                     `json:"cephMdsNodes"`
	CephRgwNodes            []string                     `json:"cephRgwNodes"`
	CephNodesSSH            map[string]kubessh.LoginInfo `json:"cephNodesSSH"`
	DockerStorage           map[string]storagePlan       `json:"dockerStorage"`
	EntryURL                string                       `json:"entryURL"`
	CephDeployIP            string                       `json:"cephDeployIP"`
	UseSwap                 bool                         `json:"useSwap"`
}

//全局变量 只能在cluster包内使用
var currentClusterStatus = Status{
	HealthState: stateUnhealthy,
}

//GetStatus 获取当前集群状态
func GetStatus() Status {
	return currentClusterStatus
}

//SetStatus 设置集群状态 仅为测试提供 不修改运行文件
func SetStatus(newStatus Status) {
	currentClusterStatus = newStatus

	//runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return
}

//Init 初始化集群信息
func Init() {
	var recoverData Status

	err := runconf.Read(runconf.DataTypeCurrentClusterStatus, &recoverData)
	if err == nil {
		currentClusterStatus = recoverData
	}

	if len(currentClusterStatus.NodesSSH) == 0 {
		currentClusterStatus.NodesSSH = make(map[string]kubessh.LoginInfo, 0)
	}

	initMoulesInfo()

	return
}

//构建出与kubeinstall一致的工作目录
func createWorkDir(s *ssh.Client) (string, error) {
	mkdirCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`sudo mkdir -p %s`, config.GetDownloadsDir()),
		ErrorTags:  []string{"sudo: sorry, you must have a tty to run sudo"},
	}

	output, err := kubessh.ClientRunCMD(session.InvaildSessionID, s, mkdirCMD)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---创建缓存文件夹失败---", output)

		return output, err
	}

	mkdirCMD = cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`sudo mkdir -p %s`, config.GetCacheDir()),
		ErrorTags:  []string{"sudo: sorry, you must have a tty to run sudo"},
	}

	output, err = kubessh.ClientRunCMD(session.InvaildSessionID, s, mkdirCMD)

	return output, err
}

//CheckInstallPlan 参数检查
func (p *InstallPlan) CheckInstallPlan(sessionID int) (string, error) {
	var finalMsg string
	var finalErr error

	context := state.Context{
		State: state.Running,
	}

	for _, sshInfo := range p.MachineSSHSet {
		sshClient, err := sshInfo.ConnectToHost()
		if err != nil {
			finalMsg += fmt.Sprintf(`主机:%s SSH信息验证失败!
`, sshInfo.HostAddr)
			continue
		}

		//各个节点创建工作目录$HOME/kubeinstall/downloads $HOME/kubeinstall/cache 2个目录
		output, err := createWorkDir(sshClient)

		sshClient.Close()

		if err != nil {
			finalMsg += fmt.Sprintf(`主机:%s 创建缓存文件夹失败:%s!
`,
				sshInfo.HostAddr,
				output,
			)
			//finalErr = err
		}
	}

	errCode := p.checkEtcd()
	if errCode != msg.Success {
		finalMsg += msg.GetErrMsg(errCode)
	}

	errCode = p.checkEntry()
	if errCode != msg.Success {
		finalMsg += msg.GetErrMsg(errCode)
	}

	errCode = p.checkYUM()
	if errCode != msg.Success {
		finalMsg += msg.GetErrMsg(errCode)
	}

	var errMsg = ""

	errCode, errMsg = p.checkDocker()
	if errCode != msg.Success {
		finalMsg += errMsg
	}

	//errCode = p.checkCeph()
	//if errCode != msg.Success {
	//	finalMsg += msg.GetErrMsg(errCode)
	//}

	errCode = p.checkMaster()
	if errCode != msg.Success {
		finalMsg += msg.GetErrMsg(errCode)
	}

	if finalMsg != "" {
		finalErr = errors.New(finalMsg)
	}

	//暂时只支持单节点 假设只有ha的错误则认为成功
	if finalMsg == msg.GetErrMsg(msg.MasterNotHighAvailable) {
		finalErr = nil
	}

	logdebug.Println(logdebug.LevelInfo, "----校验结果----", finalMsg)

	context.State = state.Complete
	if finalErr != nil {
		context.State = state.Error
	}
	context.Stdout = finalMsg
	context.SchedulePercent = "100%"

	state.Update(sessionID, context)

	return finalMsg, finalErr
}

func cutIPFromURL(url string) string {
	ipStart := strings.Split(url, "http://")

	ipBody := strings.Split(ipStart[1], ".")

	ipEnd := strings.Split(ipBody[3], ":")

	ip := ""

	ip += ipBody[0] + "." + ipBody[1] + "." + ipBody[2] + "." + ipEnd[0]

	return ip
}

func sendRequest2K8scc(sessionID int, url string, cmdInfo cmd.ExecInfo) (string, error) {
	var (
		resp   msg.Response
		output string
		errMsg string
	)

	context := state.Context{
		State: state.Running,
	}

	state.Update(sessionID, context)

	reqExecCMD := msg.Request{
		Type:    msg.POST,
		URL:     url,
		Content: cmdInfo,
	}

	reqNodeQuery := msg.Request{
		Type: msg.PUT,
	}

	nodeState := node.NodeContext{}

	ip := cutIPFromURL(url)
	cmdPrefix := fmt.Sprintf(`[%s]# `, ip)

	//原始命令 先写入一个命令body 以便前端知道当前卡在那个命令
	//更新至全局进度条
	context.Stdout += cmdPrefix + cmdInfo.CMDContent + "\n"
	context.State = state.Running
	context.SchedulePercent = cmdInfo.SchedulePercent
	context.Tips = cmdInfo.Tips
	state.Update(sessionID, context)

	//更新至node进度条
	nodeState.Context.Stdout = cmdPrefix + cmdInfo.CMDContent + "\n"
	nodeState.Context.Tips = cmdInfo.Tips
	reqNodeQuery.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + ip
	reqNodeQuery.Content = nodeState
	reqNodeQuery.SendRequestByJSON()

	//出错 写入error事件

	//logdebug.Println(logdebug.LevelInfo, "----reqExecCMD-----", reqExecCMD)
	data, err := reqExecCMD.SendRequestByJSON()
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		context.Tips += "...安装失败!"
		state.Update(sessionID, context)

		nodeState.Context.ErrMsg = err.Error()
		nodeState.Context.State = state.Error
		nodeState.Context.Tips = context.Tips
		reqNodeQuery.Content = nodeState
		reqNodeQuery.SendRequestByJSON()

		return err.Error(), err
	}

	json.Unmarshal(data, &resp)

	//获取屏幕输出存在output里
	context.Stdout = resp.Output
	nodeState.Context.Stdout = resp.Output
	reqNodeQuery.Content = nodeState

	output += resp.Output
	if resp.Result != true {
		errMsg += resp.ErrorMsg

		context.State = state.Error
		context.ErrMsg = resp.ErrorMsg
		context.Tips += "...安装失败!"
		state.Update(sessionID, context)

		nodeState.Context.ErrMsg = resp.ErrorMsg
		nodeState.Context.State = state.Error
		nodeState.Context.Tips = context.Tips
		reqNodeQuery.Content = nodeState
		reqNodeQuery.SendRequestByJSON()

		logdebug.Println(logdebug.LevelError, "---cmd:", reqNodeQuery.Content, "执行失败----")

		return output, errors.New(errMsg)
	}

	//命令执行成功 更新output
	state.Update(sessionID, context)
	reqNodeQuery.SendRequestByJSON()

	return output, nil
}

//顺序发送列表中的命令给K8scc执行
func sendCMDToK8scc(sessionID int, url string, cmdList []cmd.ExecInfo) (string, error) {
	var (
		output string
		err    error
	)

	for _, cmdInfo := range cmdList {
		logdebug.Println(logdebug.LevelInfo, url, cmdInfo)

		output, err = sendRequest2K8scc(sessionID, url, cmdInfo)
		if err != nil {
			return output, err
		}
	}

	return output, nil
}

func sshSendFile(client *ssh.Client, srcFile string, destPath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fmt.Println("-----newClient----", err)
		return err
	}

	defer sftpClient.Close()

	//"yum install -y vsftpd"
	//return kubessh.SftpSend(sftpClient, srcFile, destPath)
	return kubessh.ExecSendFileCMD(client, sftpClient, srcFile, destPath)
}

func getNowUTCTime() string {
	now := time.Now()
	year, mon, day := now.UTC().Date()
	hour, min, sec := now.UTC().Clock()
	zone, _ := now.UTC().Zone()
	utcTime := ""

	utcTime = fmt.Sprintf(`UTC time is %d-%d-%d %02d:%02d:%02d %s\n`,
		year,
		mon,
		day,
		hour,
		min,
		sec,
		zone,
	)

	return utcTime
}

func remoteExecCMD(sessionID int, client *ssh.Client, cmdList []cmd.ExecInfo) error {
	var err error

	for _, cmdInfo := range cmdList {
		_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

//在参数列表中提到的所有主机上批量执行命令
func multiExecCMD(sessionID int, machineSSHSet map[string]kubessh.LoginInfo, cmdList []cmd.ExecInfo) {
	//logdebug.Println(logdebug.LevelInfo, "------清理各个节点配置的repo------")
	var (
		client   *ssh.Client
		err      error
		finalErr error
	)

	wg := &sync.WaitGroup{}

	wg.Add(len(machineSSHSet))

	for _, sshInfo := range machineSSHSet {
		logdebug.Println(logdebug.LevelInfo, "-----清理主机-----", sshInfo, cmdList)

		go func(nodeSSH kubessh.LoginInfo) {

			defer wg.Done()

			client, err = sshInfo.ConnectToHost()
			if err != nil {

				return
			}

			defer client.Close()

			err = remoteExecCMD(sessionID, client, cmdList)
			if err != nil {
				return
			}

		}(sshInfo)

		if err != nil {
			finalErr = err
		}
	}

	if finalErr != nil {
		context := state.Context{
			State: state.Error,
		}

		state.Update(sessionID, context)
	}

	wg.Wait()

	return
}

//Kill 强制停止指定主机正在执行的k8scc命令
func (p *InstallPlan) Kill() (string, error) {
	resp := msg.Response{}
	var output string
	var finalErr error
	var errMsg string

	for _, hostSSH := range p.MachineSSHSet {

		req := msg.Request{
			Type: msg.DELETE,
			URL:  "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath,
			//Content: "",
		}
		data, err := req.SendRequestByJSON()
		if err != nil {
			return err.Error(), err
		}

		json.Unmarshal(data, &resp)
		output += resp.Output
		if resp.Result != true {
			errMsg += resp.ErrorMsg
		}
	}

	if errMsg != "" {
		finalErr = errors.New(errMsg)

		return output, finalErr
	}

	//output += fmt.Sprintf(`%s 命令列表:%s执行成功!`, url, cmdList)

	return output, nil
}

func addCMDHostAddr(cmdList []cmd.ExecInfo, cmdHostAddr string) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range cmdList {
		cmdInfo.CMDHostAddr = cmdHostAddr
		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func addSchedulePercent(cmdList []cmd.ExecInfo, percent string) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range cmdList {
		cmdInfo.SchedulePercent = percent
		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

//新开启一个node的进度
func newNodeScheduler(step, nodeIP, scheduler, tips string) {
	var (
		nodeState node.NodeContext
		req       msg.Request
	)

	nodeState.HostIP = nodeIP
	nodeState.Step = step
	nodeState.Context.State = state.Running
	nodeState.Context.SchedulePercent = scheduler
	nodeState.Context.Tips = tips
	req.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery"
	req.Type = msg.POST
	req.Content = nodeState
	req.SendRequestByJSON()

	return
}

//更新node进度
func updateNodeScheduler(nodeIP, scheduler, taksState, tips string) {
	var (
		nodeState node.NodeContext
		req       msg.Request
	)

	req.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + nodeIP
	req.Type = msg.PUT
	nodeState.Context.SchedulePercent = scheduler
	nodeState.Context.State = taksState
	nodeState.Context.Tips = tips
	req.Content = nodeState
	req.SendRequestByJSON()

	return
}

//配置error进度
func setErrNodeScheduler(nodeIP, scheduler, errMsg, tips string) {
	var (
		nodeState node.NodeContext
		req       msg.Request
	)

	req.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + nodeIP
	req.Type = msg.PUT
	nodeState.Context.SchedulePercent = scheduler
	nodeState.Context.State = state.Error
	nodeState.Context.Tips = tips
	nodeState.Context.ErrMsg = errMsg
	req.Content = nodeState
	req.SendRequestByJSON()

	return
}
