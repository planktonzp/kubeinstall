package cluster

import (
	//"kubeinstall/src/kubessh"
	//"os"
	//"fmt"
	//"kubeinstall/src/backup"
	"kubeinstall/src/logdebug"
	//"os"
	//"encoding/json"
	"fmt"
	//"github.com/pkg/errors"
	"kubeinstall/src/runconf"
	//"kubeinstall/src/client"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	//"kubeinstall/src/msg"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/state"
	"strconv"
	"strings"
	"sync"
	"time"
)

const rpmsInstallCMD = "yum install -y docker kubelet kubeadm kubectl kubernetes-cni"

const (
	fastSetupCfgPath      = "/opt/k8scc/conf/fast.properties"
	swaggerCfgPath        = "/opt/k8scc/swagger-ui/dist/index.html"
	kubeWorkerCfgPath     = "/etc/sysconfig/kubeworker/kubeworker.cfg"
	defaultSubNet         = "172.17"
	hostKey               = "$(HOST)"
	portKey               = "$(PORT)"
	fastSetupConfPathKey  = "$(FASTSETUP_CONF_PATH)"
	indexHTMLPathKey      = "$(INDEX_HTML_PATH)"
	subNetKey             = "$(SUBNET)"
	kubeWorkerConfPathKey = "$(KUBEWORKER_CONF_PATH)"
)

//yum相关命令配置文件
type rpmsCMDConf struct {
	InstallFastSetup cmd.ExecInfo   `json:"installK8scc"`
	ModifyFastSetup  []cmd.ExecInfo `json:"modifyFastSetup"`
	StartFastSetup   cmd.ExecInfo   `json:"startFastSetup"`
	InstallEntry     []cmd.ExecInfo `json:"installEntry"`

	//反操作命令集合
	UninstallRPMsCMDList []cmd.ExecInfo `json:"uninstallRPMsCMDList"`
}

//RpmsConf 用户定制的关于rpm包安装的参数
type RpmsConf struct {
	EntryPort  int    `json:"entryPort"`  //entry服务端口
	EntryHeart int    `json:"entryHeart"` //entry心跳 单位秒
	EntryHost  string `json:"entryHost"`  //在哪里安装entry
}

//var sessionID int

//读取命令json 修改k8scc启动命令 将修改后的命令发送给远端主机
func dynamicCreateK8sccCfg(innerIP string, cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	oldIP := hostKey
	oldPort := portKey
	oldFastSetupConfPath := fastSetupConfPathKey
	oldIndexHTMLPath := indexHTMLPathKey
	oldSubNet := subNetKey
	oldKubeWorkerConfPath := kubeWorkerConfPathKey
	newCmdList := []cmd.ExecInfo{}

	newPort := config.GetWorkerPort()

	//替换所有命令的6类key
	for _, cmd := range cmdList {
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldIP, innerIP, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldPort, newPort, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldFastSetupConfPath, fastSetupCfgPath, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldIndexHTMLPath, swaggerCfgPath, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldSubNet, defaultSubNet, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, oldKubeWorkerConfPath, kubeWorkerCfgPath, -1)

		newCmdList = append(newCmdList, cmd)
	}

	return newCmdList
}

func installK8scc(sessionID int, innerIP string, sshInfo kubessh.LoginInfo) error {
	conf := rpmsCMDConf{}

	context := state.Context{
		State: state.Running,
		Tips:  "正在安装k8scc!",
	}

	state.Update(sessionID, context)

	runconf.Read(runconf.DataTypeRpmsCMDConf, &conf)

	//logdebug.Println(logdebug.LevelInfo, "---rpm---json---", conf)
	//使用前端发过来的IP + json文件中的命令模板 组合出新的命令发给远端主机
	newCmdList := dynamicCreateK8sccCfg(innerIP, conf.ModifyFastSetup)

	newNodeScheduler(Step3, sshInfo.HostAddr, "30%", "正在安装k8scc!")

	sshClient, err := sshInfo.ConnectToHost()
	if err != nil {
		//调用SSH执行命令接口 或者 发送给k8scc的接口 不需要更新状态 其余自行更新状态
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(sshInfo.HostAddr, "30%", err.Error(), "连接主机失败!")

		return err
	}

	defer sshClient.Close()

	conf.InstallFastSetup.SchedulePercent = "30%"

	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.InstallFastSetup)
	if err != nil {
		return err
	}

	updateNodeScheduler(sshInfo.HostAddr, "30%", state.Running, "正在配置k8scc!")

	//发送修改启动参数的命令集合给主机
	newCmdList = addSchedulePercent(newCmdList, "30%")
	for _, cmdInfo := range newCmdList {
		//cmd1
		_, err = kubessh.ClientRunCMD(sessionID, sshClient, cmdInfo)
		if err != nil {
			return err
		}
	}

	updateNodeScheduler(sshInfo.HostAddr, "30%", state.Running, "正在启动k8scc!")

	//cmd1
	conf.StartFastSetup.SchedulePercent = "30%"
	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.StartFastSetup)
	if err != nil {
		return err
	}

	return err
}

func (p *InstallPlan) installCoreRPMs(sessionID int) error {
	workerServerPort := config.GetWorkerPort()
	conf := kubeWorkerCMDConf{}
	var err error
	var finalErr error

	context := state.Context{
		State: state.Running,
	}

	state.Update(sessionID, context)

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//logdebug.Println(logdebug.LevelInfo, "-----读取配置文件json-----", conf)

	wg := &sync.WaitGroup{}

	wg.Add(len(p.MachineSSHSet))

	//所有协程的worker都安装完毕后 返回(外层需要所有worker安装完成)
	for ip := range p.MachineSSHSet {
		go func(ip string) {
			updateNodeScheduler(ip, "60%", state.Running, "正在安装容器核心组件!")

			defer wg.Done()

			workerURL := "http://" + ip + ":" + workerServerPort + workerServerPath
			//cmd 2
			conf.InstallCoreCMDList = addSchedulePercent(conf.InstallCoreCMDList, "60%")
			_, err = sendCMDToK8scc(sessionID, workerURL, conf.InstallCoreCMDList)

		}(ip)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	//有一个出错则最终认为出错
	if finalErr != nil {
		context.State = state.Error

		state.Update(sessionID, context)
	}

	return finalErr
}

func perRequestK8scc(hostAddr string) {
	//TODO: 探测  预访问 以保证时序
	time.Sleep(time.Second * 1)

	logdebug.Println(logdebug.LevelInfo, "------探测k8scc-----")

	return
}

/*

kernel.pid_max = 4194303
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-arptables = 1
net.ipv4.ip_forward = 1

*/
func (p *InstallPlan) modifySysctlConf(sessionID int) error {
	kernelPidMAX := "kernel.pid_max = 4194303"
	netBridgeIP6Tables := "net.bridge.bridge-nf-call-ip6tables = 1"
	netBridgeIPTables := "net.bridge.bridge-nf-call-iptables = 1"
	netBridgeARPTables := "net.bridge.bridge-nf-call-arptables = 1"
	netIPV4Forward := "net.ipv4.ip_forward = 1"

	cfgSet := []string{kernelPidMAX, netBridgeIP6Tables, netBridgeIPTables, netBridgeARPTables, netIPV4Forward}

	checkCMDInfo := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`cat /etc/sysctl.conf`),
		SchedulePercent: "90%",
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(p.MachineSSHSet))

	var err error
	var resp string
	var finalErr error

	//cmd 3
	for _, machineSSH := range p.MachineSSHSet {
		go func(machineSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(machineSSH.HostAddr, "90%", state.Running, "正在配置sysctl.conf!")

			url := "http://" + machineSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			resp, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{checkCMDInfo})
			if err != nil {
				return
			}

			err = addRules(sessionID, url, resp, "/etc/sysctl.conf", cfgSet)
			if err != nil {
				return
			}

			checkCMDInfo.CMDContent = "cat /etc/profile"

			resp, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{checkCMDInfo})
			if err != nil {
				return
			}

			err = addRules(sessionID, url, resp, "/etc/profile", []string{"swapoff -a"})
			if err != nil {
				return
			}

			updateNodeScheduler(machineSSH.HostAddr, "100%", state.Complete, "配置sysctl.conf完成!")

		}(machineSSH)

		if err != nil {
			finalErr = err
		}

	}

	wg.Wait()

	return finalErr
}

func addRules(sessionID int, url, src, filePath string, cfgSet []string) error {
	var err error

	for _, cfg := range cfgSet {
		if strings.Contains(src, cfg) {
			//已经添加过 无需添加
			continue
		}
		cmdInfo := cmd.ExecInfo{
			CMDContent:      fmt.Sprintf(`echo "%s" >>%s`, cfg, filePath),
			SchedulePercent: "90%",
			Tips:            "正在配置sysctl.conf!",
		}
		//cmd 3
		_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
		if err != nil {
			return err
		}
	}

	return err
}

func (r *RpmsConf) dynamicModifyInstallEntryCMDList(oldCMDList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range oldCMDList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(ENTRY_PORT)", strconv.Itoa(r.EntryPort), -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(ENTRY_HEART)", strconv.Itoa(r.EntryHeart), -1)

		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func (p *InstallPlan) installEntry(sessionID int) error {
	conf := rpmsCMDConf{}

	if currentClusterStatus.EntryURL != "" {
		logdebug.Println(logdebug.LevelInfo, "---已经安装了entry---")

		return nil
	}

	runconf.Read(runconf.DataTypeRpmsCMDConf, &conf)

	if p.RpmsCfg.EntryHost == "" {
		logdebug.Println(logdebug.LevelInfo, "---未配置entry选项 伪装默认值")
		p.RpmsCfg.EntryPort = config.GetEntryPort()
		p.RpmsCfg.EntryHeart = config.GetEntryHeart()

		//任意找一个主机安装
		for k := range p.MachineSSHSet {
			p.RpmsCfg.EntryHost = k

			break
		}
	}

	logdebug.Println(logdebug.LevelInfo, "---entry request---", p.RpmsCfg)

	newCMDList := p.RpmsCfg.dynamicModifyInstallEntryCMDList(conf.InstallEntry)
	entryHostURL := "http://" + p.MachineSSHSet[p.RpmsCfg.EntryHost].HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	logdebug.Println(logdebug.LevelInfo, "---动态修改后 安装entry CMD----", newCMDList, entryHostURL)

	updateNodeScheduler(p.RpmsCfg.EntryHost, "70%", state.Running, "正在entry!")
	//entryHostURL := "http://" + p.MachineSSHSet[p.RpmsCfg.EntryHost].HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	sendCMDToK8scc(sessionID, entryHostURL, newCMDList)
	currentClusterStatus.EntryURL = "http://" + p.RpmsCfg.EntryHost + ":" + strconv.Itoa(p.RpmsCfg.EntryPort)
	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return nil

}

//InstallRPMS 安装RPM包 安装fastSetup(kubeworker)
func (p *InstallPlan) InstallRPMS(sessionID int) error {
	var (
		err      error
		finalErr error
		context  state.Context
	)

	logdebug.Println(logdebug.LevelInfo, "-------安装RPM包-----")

	wg := &sync.WaitGroup{}
	wg.Add(len(p.MachineSSHSet))

	//首先通过SSH安装启动fastSetup(kubeworker) 后续操作都向fast请求
	//遍历顺序不确定 所以 一个出错之后 剩下的继续
	for innerIP, sshInfo := range p.MachineSSHSet {
		go func(innerIP string, sshInfo kubessh.LoginInfo) error {
			defer wg.Done()
			//cmd 1
			err = installK8scc(sessionID, innerIP, sshInfo)
			if err != nil {
				return err
			}

			perRequestK8scc(sshInfo.HostAddr)

			return nil
		}(innerIP, sshInfo)

		//一旦出错 则已经被置为ERROR
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	if finalErr != nil {
		//多协程中存在一个或多个错误 为了防止最后一个正确 覆盖了状态 ，强行添加error
		context.State = state.Error

		state.Update(sessionID, context)

		return finalErr
	}

	//install Other RPMs by http 需要fastSetup程序 关闭防火墙
	//一旦出错 则已经被置为ERROR
	err = p.installCoreRPMs(sessionID)
	if err != nil {
		return err
	}

	err = p.installEntry(sessionID)
	if err != nil {
		return err
	}

	//一旦出错 则已经被置为ERROR
	p.modifySysctlConf(sessionID)
	if err != nil {
		return err
	}

	//流程完全没有出错 complete
	context.State = state.Complete
	context.SchedulePercent = "100%"

	state.Update(sessionID, context)

	return err
}

func (p *InstallPlan) checkEntry() msg.ErrorCode {
	if _, ok := p.MachineSSHSet[p.RpmsCfg.EntryHost]; !ok {
		logdebug.Println(logdebug.LevelError, "entry指定的host不在主机列表中!")

		return msg.EntrySSHNotSpecified
	}

	return msg.Success
}

//RemoveRPMs 移除在各个节点上已经安装的k8s docker相关的rpm (k8scc 应该保留)
func (p *InstallPlan) RemoveRPMs(moduleSessionID int) (string, error) {
	var conf rpmsCMDConf

	runconf.Read(runconf.DataTypeRpmsCMDConf, &conf)

	multiExecCMD(moduleSessionID, p.MachineSSHSet, conf.UninstallRPMsCMDList)

	return "移除核心RPM包", nil
}
