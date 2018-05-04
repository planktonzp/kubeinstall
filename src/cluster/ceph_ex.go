package cluster

import (
	//"github.com/pkg/sftp"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	//"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/runconf"
	"kubeinstall/src/state"
	//"math/cmplx"
	"strings"
	"sync"
)

func (c *CephConf) yumMakeCache(sessionID int, client *ssh.Client, host string) error {
	repoFilePath := config.GetDownloadsDir() + "Ftp.repo"

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	//各个节点创建工作目录$HOME/kubeinstall/downloads $HOME/kubeinstall/cache 2个目录
	_, err = createWorkDir(client)
	if err != nil {
		return err
	}

	conf := yumCMDConf{}
	//yum clean all && yum makecache
	runconf.Read(runconf.DataTypeYumCMDConf, &conf)
	conf.BackupOldRepos.CMDHostAddr = host

	//cmd 8 外层一个循环 取最后的一次进度
	newNodeScheduler(StepEX8, host, "15%", "正在发送repo文件到主机...")

	kubessh.ExecSendFileCMD(client, sftpClient, repoFilePath, etcYumReposDPath)

	logdebug.Println(logdebug.LevelInfo, "已发送Ftp.repo至主机", host)

	updateNodeScheduler(host, "30%", state.Running, "正在执行yum makecache...")

	conf.YUMMakeCacheCMDList = addSchedulePercent(conf.YUMMakeCacheCMDList, "30%")
	for _, cmdInfo := range conf.YUMMakeCacheCMDList {
		cmdInfo.CMDHostAddr = host
		_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return err
}

//应用yum源 占总进度30% 波及各个node均占30%
func (c *CephConf) applyYumStorage(sessionID int) error {
	var (
		client   *ssh.Client
		err      error
		finalErr error
	)
	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			logdebug.Println(logdebug.LevelInfo, "---------------nodeSSH--------", nodeSSH.HostAddr)

			defer wg.Done()

			client, err = nodeSSH.ConnectToHost()
			if err != nil {
				return
			}

			defer client.Close()

			err = c.yumMakeCache(sessionID, client, nodeSSH.HostAddr)

			//出错后屏幕输入已经更新 记录曾经出错 外层调用
			if err != nil {
				finalErr = err
			}

		}(nodeSSH)
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) installK8scc(sessionID int, innerIP string, sshInfo kubessh.LoginInfo) error {
	conf := rpmsCMDConf{}

	updateNodeScheduler(innerIP, "40%", state.Running, "正在尝试连接主机...")

	context := state.Context{
		State: state.Running,
	}

	state.Update(sessionID, context)

	runconf.Read(runconf.DataTypeRpmsCMDConf, &conf)

	//logdebug.Println(logdebug.LevelInfo, "---rpm---json---", conf)
	//使用前端发过来的IP + json文件中的命令模板 组合出新的命令发给远端主机
	newCmdList := dynamicCreateK8sccCfg(innerIP, conf.ModifyFastSetup)

	sshClient, err := sshInfo.ConnectToHost()
	if err != nil {
		//调用SSH执行命令接口 或者 发送给k8scc的接口 不需要更新状态 其余自行更新状态
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(innerIP, "40%", err.Error(), "正在尝试连接主机...失败")

		return err
	}

	defer sshClient.Close()

	updateNodeScheduler(innerIP, "50%", state.Running, "正在安装k8scc")
	conf.InstallFastSetup.SchedulePercent = "50%"
	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.InstallFastSetup)
	if err != nil {
		//setErrNodeScheduler(innerIP, "50%", err.Error(), "正在安装k8scc...失败")

		return err
	}

	updateNodeScheduler(innerIP, "60%", state.Running, "正在修改k8scc启动参数")

	//发送修改启动参数的命令集合给主机
	newCmdList = addSchedulePercent(newCmdList, "60%")
	for _, cmdInfo := range newCmdList {
		//cmd1
		_, err = kubessh.ClientRunCMD(sessionID, sshClient, cmdInfo)
		if err != nil {
			return err
		}
	}

	updateNodeScheduler(innerIP, "70%", state.Running, "正在启动k8scc")
	//cmd1
	conf.StartFastSetup.SchedulePercent = "70%"
	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.StartFastSetup)
	if err != nil {
		return err
	}

	return err
}

func (c *CephConf) installCoreRPMs(sessionID int) error {
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

	wg.Add(len(c.NodesSSHSet))

	//所有协程的worker都安装完毕后 返回(外层需要所有worker安装完成)
	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(nodeSSH.HostAddr, "70%", state.Running, "正在安装ceph核心组件")

			workerURL := "http://" + nodeSSH.HostAddr + ":" + workerServerPort + workerServerPath
			//cmd 2
			conf.InstallCephCMDList = addSchedulePercent(conf.InstallCephCMDList, "70%")
			_, err = sendCMDToK8scc(sessionID, workerURL, conf.InstallCephCMDList)

		}(nodeSSH)

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

func (c *CephConf) addRules(sessionID int, url, src string, cfgSet []string) error {
	var err error

	for _, cfg := range cfgSet {
		if strings.Contains(src, cfg) {
			//已经添加过 无需添加
			continue
		}
		cmdInfo := cmd.ExecInfo{
			CMDContent:      fmt.Sprintf(`echo "%s" >>/etc/sysctl.conf`, cfg),
			SchedulePercent: "50%",
		}
		//cmd 3
		_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
		if err != nil {
			return err
		}
	}

	return err
}

func (c *CephConf) modifySysctlConf(sessionID int) error {
	kernelPidMAX := "kernel.pid_max = 4194303"
	netBridgeIP6Tables := "net.bridge.bridge-nf-call-ip6tables = 1"
	netBridgeIPTables := "net.bridge.bridge-nf-call-iptables = 1"
	netBridgeARPTables := "net.bridge.bridge-nf-call-arptables = 1"
	netIPV4Forward := "net.ipv4.ip_forward = 1"

	cfgSet := []string{kernelPidMAX, netBridgeIP6Tables, netBridgeIPTables, netBridgeARPTables, netIPV4Forward}

	checkCMDInfo := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`cat /etc/sysctl.conf`),
		SchedulePercent: "70%",
	}

	//cmd 3
	for _, machineSSH := range c.NodesSSHSet {
		updateNodeScheduler(machineSSH.HostAddr, "70%", state.Running, "正在配置系统参数")

		url := "http://" + machineSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
		resp, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{checkCMDInfo})
		if err != nil {
			return err
		}

		err = c.addRules(sessionID, url, resp, cfgSet)
		if err != nil {
			return err
		}

		updateNodeScheduler(machineSSH.HostAddr, "70%", state.Running, "正在应用系统参数")
	}

	return nil
}

//应用核心rpm包 总进度70% 波及各个node均70%
func (c *CephConf) installRPMS(sessionID int) error {
	var (
		err      error
		finalErr error
		context  state.Context
	)

	logdebug.Println(logdebug.LevelInfo, "-------安装RPM包-----")

	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	//首先通过SSH安装启动fastSetup(kubeworker) 后续操作都向fast请求
	//遍历顺序不确定 所以 一个出错之后 剩下的继续
	//外层25% -50%
	for innerIP, sshInfo := range c.NodesSSHSet {
		go func(innerIP string, sshInfo kubessh.LoginInfo) error {
			defer wg.Done()
			//cmd 1 35%
			err = c.installK8scc(sessionID, innerIP, sshInfo)
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
	err = c.installCoreRPMs(sessionID)
	if err != nil {
		return err
	}

	//一旦出错 则已经被置为ERROR
	c.modifySysctlConf(sessionID)
	if err != nil {
		return err
	}

	//流程完全没有出错 complete
	//context.State = state.Complete
	context.SchedulePercent = "70%"

	state.Update(sessionID, context)

	return err
}

func (c *CephConf) reCommunicateEachOther(sessionID int) error {

	err := c.modifyHosts(sessionID)
	if err != nil {
		return err
	}

	//配置各个ceph节点间互信
	err = c.setCommunicateEachOther(sessionID)
	if err != nil {
		return err
	}

	return nil
}

func (c *CephConf) reTimeSync(sessionID int) error {

	err := c.commentOtherServerForNtpCfg(sessionID)
	if err != nil {
		return err
	}

	err = c.addNewServerForNtpCfg(sessionID)

	return err
}

//更新当前ceph集群 总进度100% mon osd mon/osd应区分进度
func (c *CephConf) updateCurrentCluster(sessionID int) error {
	newCfg := c

	//将现有集群的nodeSSH信息更新至新的对象中(互信、时间同步要用)
	for k, v := range currentClusterStatus.CephNodesSSH {
		newCfg.NodesSSHSet[k] = v
	}

	err := newCfg.reCommunicateEachOther(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---重新互信失败---", err.Error())

		return err
	}

	err = newCfg.reTimeSync(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---重新时间同步失败---", err.Error())

		return err
	}

	//实际上是ceph osd create 或者mon的（ceph-deloy add）
	err = c.cephNodeJoin(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---新节点加入Ceph集群失败---", err.Error())

		return err
	}

	return nil
}

func pushOldCfg2Node(n kubessh.LoginInfo) {
	//if len(currentClusterStatus.CephMonNodes) == 0 {
	//	logdebug.Println(logdebug.LevelError, "---没有ceph集群无法推送ceph配置---")
	//
	//	return
	//}
	//
	//oldMonIP := currentClusterStatus.CephMonNodes[0]
	//monSSH := currentClusterStatus.CephNodesSSH[oldMonIP]
	//
	//client, err := monSSH.ConnectToHost()
	//if err != nil {
	//	return
	//}
	//defer client.Close()
	//
	//sftpClient, err := sftp.NewClient(client)
	//if err != nil {
	//	return
	//}
	//
	//defer sftpClient.Close()
	//
	//kubessh.SftpDownload(sftpClient, cephConfWorkDir+cephConfName, config.GetDownloadsDir())
	//
	////kubessh.SftpDownload()

	return
}

func addMon2Cluster(sessionID int, n kubessh.LoginInfo) {
	//最初执行mon init的IP
	deployMonIP := currentClusterStatus.CephDeployIP
	//monSSH := currentClusterStatus.CephNodesSSH[deployMonIP]
	monURL := "http://" + deployMonIP + ":" + config.GetWorkerPort() + workerServerPath

	newNodeURL := "http://" + n.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	cmdInfo := cmd.ExecInfo{
		CMDContent:  "hostname",
		CMDHostAddr: n.HostAddr,
	}

	output, _ := sendCMDToK8scc(sessionID, newNodeURL, []cmd.ExecInfo{cmdInfo})

	h := strings.Split(output, "\n")
	newNodeHostname := h[0]

	cmdInfo = cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`cd /etc/ceph/ && ceph-deploy mon add %s`, newNodeHostname),
	}

	updateNodeScheduler(deployMonIP, "80%", state.Running, "正在添加mon至集群...!")

	sendCMDToK8scc(sessionID, monURL, []cmd.ExecInfo{cmdInfo})

	return
}

func addOsd2Cluster(sessionID int, n kubessh.LoginInfo, r CephRole) {
	deployMonIP := currentClusterStatus.CephDeployIP
	//monSSH := currentClusterStatus.CephNodesSSH[deployMonIP]
	monURL := "http://" + deployMonIP + ":" + config.GetWorkerPort() + workerServerPath

	newNodeURL := "http://" + n.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	cmdInfo := cmd.ExecInfo{
		CMDContent:  "hostname",
		CMDHostAddr: n.HostAddr,
	}

	output, _ := sendCMDToK8scc(sessionID, newNodeURL, []cmd.ExecInfo{cmdInfo})

	h := strings.Split(output, "\n")
	newNodeHostname := h[0]

	cmdList := []cmd.ExecInfo{}
	for data, journal := range r.Attribute.OSDMediasPath {
		activateCMD := cmd.ExecInfo{}

		if journal != "" {
			journal = ":" + journal
			activateCMD = cmd.ExecInfo{
				//ceph-deploy osd activate selfHostname:/dev/sdb:/dev/sda4
				CMDContent:      fmt.Sprintf(`cd /etc/ceph/ && ceph-deploy --overwrite-conf osd activate %s:%s%s`, newNodeHostname, data, journal),
				SchedulePercent: "90%",
			}
		}

		cmdInfo := cmd.ExecInfo{
			//ceph-deploy osd prepare selfHostname:/dev/sdb:/dev/sda4
			CMDContent: fmt.Sprintf(`cd /etc/ceph/ && ceph-deploy --overwrite-conf osd create %s:%s%s`, newNodeHostname, data, journal),
		}

		cmdList = append(cmdList, cmdInfo)

		cmdList = append(cmdList, activateCMD)
	}

	updateNodeScheduler(deployMonIP, "90%", state.Running, "正在添加osd至集群...!")

	sendCMDToK8scc(sessionID, monURL, cmdList)

	return
}

func (c *CephConf) cephNodeJoin(sessionID int) error {
	//logdebug.Println(logdebug.LevelInfo, "---新增ceph节点---")

	for k, v := range c.RolesSet {
		nodeSSH := c.NodesSSHSet[k]

		pushOldCfg2Node(nodeSSH)
		//push old cfg to node

		if isRole(v.Types, cephRoleMON) {
			//mon add to cluster
			addMon2Cluster(sessionID, nodeSSH)
		}

		if isRole(v.Types, cephRoleOSD) {
			//osd create
			addOsd2Cluster(sessionID, nodeSSH, v)
		}

		updateNodeScheduler(nodeSSH.HostAddr, "100%", state.Complete, "ceph节点拓展完成!")
		logdebug.Println(logdebug.LevelInfo, "----ceph节点拓展完成!-----")
	}

	return nil
}

//InstallCephEX 拓展安装ceph
func (p *InstallPlan) InstallCephEX(sessionID int) error {
	c := p.CephCfg
	var context state.Context

	//重新规划安装进度条-30%
	err := c.applyYumStorage(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---ceph 节点应用yum源失败---", err.Error())

		return err
	}

	//70%
	err = c.installRPMS(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---ceph 节点rpm包安装失败---", err.Error())

		return err
	}

	//100%
	err = c.updateCurrentCluster(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---ceph 节点rpm包安装失败---", err.Error())

		return err
	}

	context.SchedulePercent = "100%"
	context.State = state.Complete
	state.Update(sessionID, context)

	return nil
}
