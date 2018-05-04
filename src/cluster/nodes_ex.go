package cluster

import (
	"kubeinstall/src/logdebug"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	//"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"sync"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"golang.org/x/crypto/ssh"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"kubeinstall/src/state"
	"github.com/pkg/sftp"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"fmt"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"github.com/docker/docker/integration-cli/cli"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/cmd"
	"kubeinstall/src/runconf"
	"kubeinstall/src/state"
	"strings"
)

//35-45%
func (n *NodesConf) installCoreRPMs(sessionID int) error {
	workerServerPort := config.GetWorkerPort()
	conf := kubeWorkerCMDConf{}
	var err error
	var finalErr error

	context := state.Context{
		State: state.Running,
	}

	state.Update(sessionID, context)

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	logdebug.Println(logdebug.LevelInfo, "-----读取配置文件json-----", conf)

	wg := &sync.WaitGroup{}

	wg.Add(len(n.NodesIPSet))

	//所有协程的worker都安装完毕后 返回(外层需要所有worker安装完成)
	for _, nodeSSH := range n.NodesIPSet {
		go func(nodeSSH kubessh.LoginInfo) {
			updateNodeScheduler(nodeSSH.HostAddr, "40%", state.Running, "正在安装核心组件")

			defer wg.Done()

			workerURL := "http://" + nodeSSH.HostAddr + ":" + workerServerPort + workerServerPath
			//cmd 2
			conf.InstallCoreCMDList = addSchedulePercent(conf.InstallCoreCMDList, "40%")
			_, err = sendCMDToK8scc(sessionID, workerURL, conf.InstallCoreCMDList)

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

func (n *NodesConf) addRules(sessionID int, url, src string, cfgSet []string) error {
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

//45%
func (n *NodesConf) modifySysctlConf(sessionID int) error {
	kernelPidMAX := "kernel.pid_max = 4194303"
	netBridgeIP6Tables := "net.bridge.bridge-nf-call-ip6tables = 1"
	netBridgeIPTables := "net.bridge.bridge-nf-call-iptables = 1"
	netBridgeARPTables := "net.bridge.bridge-nf-call-arptables = 1"
	netIPV4Forward := "net.ipv4.ip_forward = 1"

	cfgSet := []string{kernelPidMAX, netBridgeIP6Tables, netBridgeIPTables, netBridgeARPTables, netIPV4Forward}

	checkCMDInfo := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`cat /etc/sysctl.conf`),
		SchedulePercent: "45%",
	}

	//cmd 3
	for _, machineSSH := range n.NodesIPSet {

		url := "http://" + machineSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
		resp, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{checkCMDInfo})
		if err != nil {
			return err
		}

		err = n.addRules(sessionID, url, resp, cfgSet)
		if err != nil {
			return err
		}

		updateNodeScheduler(machineSSH.HostAddr, "50%", state.Running, "正在修改系统配置")
	}

	return nil
}

func (n *NodesConf) installK8scc(sessionID int, innerIP string, sshInfo kubessh.LoginInfo) error {
	conf := rpmsCMDConf{}

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

		setErrNodeScheduler(sshInfo.HostAddr, "", err.Error(), "节点连接失败!")

		return err
	}

	defer sshClient.Close()

	conf.InstallFastSetup.SchedulePercent = "35%"

	updateNodeScheduler(sshInfo.HostAddr, "35%", state.Running, "正在安装k8scc")

	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.InstallFastSetup)
	if err != nil {
		return err
	}

	//发送修改启动参数的命令集合给主机
	newCmdList = addSchedulePercent(newCmdList, "35%")
	for _, cmdInfo := range newCmdList {
		//cmd1
		_, err = kubessh.ClientRunCMD(sessionID, sshClient, cmdInfo)
		if err != nil {
			return err
		}
	}

	//cmd1
	conf.StartFastSetup.SchedulePercent = "35%"
	_, err = kubessh.ClientRunCMD(sessionID, sshClient, conf.StartFastSetup)
	if err != nil {
		return err
	}

	return err
}

func (n *NodesConf) installRPMS(sessionID int) error {
	var (
		err      error
		finalErr error
		context  state.Context
	)

	logdebug.Println(logdebug.LevelInfo, "-------安装RPM包-----")

	wg := &sync.WaitGroup{}
	wg.Add(len(n.NodesIPSet))

	//首先通过SSH安装启动fastSetup(kubeworker) 后续操作都向fast请求
	//遍历顺序不确定 所以 一个出错之后 剩下的继续
	//外层25% -50%
	for innerIP, sshInfo := range n.NodesIPSet {
		go func(innerIP string, sshInfo kubessh.LoginInfo) error {
			defer wg.Done()
			//cmd 1 35%
			err = n.installK8scc(sessionID, innerIP, sshInfo)
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
	err = n.installCoreRPMs(sessionID)
	if err != nil {
		return err
	}

	//一旦出错 则已经被置为ERROR
	n.modifySysctlConf(sessionID)
	if err != nil {
		return err
	}

	//流程完全没有出错 complete
	//context.State = state.Complete
	context.SchedulePercent = "50%"

	state.Update(sessionID, context)

	return err
}

func (n *NodesConf) applyDockerRegistry(sessionID int) error {
	var (
		finalErr error
		err      error
		output   string
		wg       = &sync.WaitGroup{}
	)

	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	conf.ApplyDockerRegistryCMDList = dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, currentClusterStatus.DockerRegistryURL, "/var/lib/docker", currentClusterStatus.UseSwap)
	conf.ModifyKubeletCMDList = dynamicCreateDockerCfg(conf.ModifyKubeletCMDList, currentClusterStatus.DockerRegistryURL, "/var/lib/docker", currentClusterStatus.UseSwap)

	//logdebug.Println(logdebug.LevelInfo, "--------conf.ApplyDockerRegistryCMDList----------", conf.ApplyDockerRegistryCMDList)
	wg.Add(len(n.NodesIPSet))

	//cmd 4 仓库90% node节点25%
	for _, hostSSH := range n.NodesIPSet {
		go func(hostSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(hostSSH.HostAddr, "75%", state.Running, "正在应用docker仓库")

			//获取用户配置的docker工作目录
			dockerHomeDir, ok := n.DockerGraphs[hostSSH.HostAddr]
			if !ok {
				dockerHomeDir = config.GetDockerHomeDir()
			}

			modifyDockerCMD := fmt.Sprintf(`sudo sed -i "/OPTIONS/c OPTIONS='--graph=%s --log-driver=json-file --signature-verification=false -H tcp://0.0.0.0:28015 -H unix:///var/run/docker.sock --insecure-registry=%s'" %s`,
				dockerHomeDir,
				currentClusterStatus.DockerRegistryURL,
				dockerCfgPath)

			logdebug.Println(logdebug.LevelInfo, "应用docker仓库----URL=", modifyDockerCMD)

			conf.ApplyDockerRegistryCMDList = append(conf.ApplyDockerRegistryCMDList, cmd.ExecInfo{CMDContent: modifyDockerCMD})
			conf.ApplyDockerRegistryCMDList = addSchedulePercent(conf.ApplyDockerRegistryCMDList, "75%")

			url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			_, err = sendCMDToK8scc(sessionID, url, conf.ApplyDockerRegistryCMDList)
			if err != nil {
				return
			}

			//检查之前是不是写过
			conf.CheckKubeletConfCMD.SchedulePercent = "75%"
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{conf.CheckKubeletConfCMD})
			if output != "" {
				logdebug.Println(logdebug.LevelInfo, "------kubelet参数曾经被修改过-----")

				return
			}

			//修改kubelet参数
			conf.ModifyKubeletCMDList = addSchedulePercent(conf.ModifyKubeletCMDList, "75%")
			logdebug.Println(logdebug.LevelInfo, "------修改kubelet参数-----")
			_, err = sendCMDToK8scc(sessionID, url, conf.ModifyKubeletCMDList)

			return
		}(hostSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	if finalErr != nil {
		context := state.Context{
			State:  state.Error,
			ErrMsg: finalErr.Error(),
		}

		state.Update(sessionID, context)
	}

	return finalErr
}

func (n *NodesConf) nodesJoin(sessionID int) error {
	var (
		conf     kubeWorkerCMDConf
		context  state.Context
		finalErr error
	)

	workerPort := config.GetWorkerPort()
	err := runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	conf.NodeJoinCMDList = dynamicModifyNodeJoinCMD(conf.NodeJoinCMDList)
	conf.NodeJoinCMDList = addSchedulePercent(conf.NodeJoinCMDList, "90%")
	logdebug.Println(logdebug.LevelInfo, "-----修改后的cmd--------", conf.NodeJoinCMDList)

	wg := &sync.WaitGroup{}
	wg.Add(len(n.NodesIPSet))

	for _, nodeSSH := range n.NodesIPSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(nodeSSH.HostAddr, "90%", state.Running, "正在执行node join")

			k8sccURL := "http://" + nodeSSH.HostAddr + ":" + workerPort + workerServerPath
			_, err = sendCMDToK8scc(sessionID, k8sccURL, conf.NodeJoinCMDList)
			if err != nil {
				//出错 由send函数填写err 返回即可
				return
			}

			cephConf, cephKey := downloadCephConf()
			err := sendCephConf(sessionID, cephConf, cephKey, nodeSSH)
			if err != nil {
				return
			}

			updateNodeScheduler(nodeSSH.HostAddr, "100%", state.Complete, "正在完成node join")

			currentClusterStatus.Nodes = saveNode(currentClusterStatus.Nodes, nodeSSH.HostAddr)

			return
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	if finalErr != nil {
		return finalErr
	}

	context.State = state.Complete
	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	return finalErr
}

func (n *NodesConf) yumMakeCache(sessionID int, client *ssh.Client, host string) error {
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
	newNodeScheduler(StepEX7, host, "15%", "正在备份旧的repo")

	_, err = kubessh.ClientRunCMD(sessionID, client, conf.BackupOldRepos)
	if err != nil {
		return err
	}

	//kubessh.SftpSend(sftpClient, repoFilePath, etcYumReposDPath)
	//repoFilePath := config.GetWorkDir() + "/kubeinstall/Ftp.repo"
	repoFilePath := config.GetDownloadsDir() + "Ftp.repo"
	kubessh.ExecSendFileCMD(client, sftpClient, repoFilePath, etcYumReposDPath)
	logdebug.Println(logdebug.LevelInfo, "发送Ftp.repo至主机", host)

	updateNodeScheduler(host, "25%", state.Running, "正在发送Ftp.repo")

	conf.YUMMakeCacheCMDList = addSchedulePercent(conf.YUMMakeCacheCMDList, "25%")
	for _, cmdInfo := range conf.YUMMakeCacheCMDList {
		cmdInfo.CMDHostAddr = host
		_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return err
}

//repo文件已经有了 无需构建
func (n *NodesConf) applyYumStorage(sessionID int) error {
	var (
		client   *ssh.Client
		err      error
		finalErr error
	)
	wg := &sync.WaitGroup{}
	wg.Add(len(n.NodesIPSet))

	for _, nodeSSH := range n.NodesIPSet {
		go func(nodeSSH kubessh.LoginInfo) {
			logdebug.Println(logdebug.LevelInfo, "---------------nodeSSH--------", nodeSSH.HostAddr)

			defer wg.Done()

			client, err = nodeSSH.ConnectToHost()
			if err != nil {
				logdebug.Println(logdebug.LevelError, "---IP:", nodeSSH.HostAddr, "--ERR:", err.Error())
				return
			}

			defer client.Close()

			err = n.yumMakeCache(sessionID, client, nodeSSH.HostAddr)

			//出错后屏幕输入已经更新 记录曾经出错 外层调用
			if err != nil {
				finalErr = err
			}

		}(nodeSSH)
	}

	wg.Wait()

	return finalErr
}

//NodesJoinEX 拓展node加入集群
func (p *InstallPlan) NodesJoinEX(sessionID int) error {
	//先应用yum仓库 进度条使用yum进度条？ 重写一套函数？

	logdebug.Println(logdebug.LevelInfo, "-----拓展安装node 收到请求-----", p)
	//25%
	err := p.NodesCfg.applyYumStorage(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----拓展安装node 应用yum源失败-----", err.Error())

		return err
	}

	//50%
	err = p.NodesCfg.installRPMS(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----拓展安装node rpm包安装失败-----", err.Error())

		return err
	}

	//75%
	//选择存储类型......
	err = p.NodesCfg.applyDockerRegistry(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----拓展安装node 应用docker仓库失败-----", err.Error())

		return err
	}

	//100%
	err = p.NodesCfg.nodesJoin(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----拓展安装noden join失败-----", err.Error())

		return err
	}

	//保存nodeSSH信息
	for ip, nodeSSH := range p.NodesCfg.NodesIPSet {
		//没有通过nodeJoin 仅通过ex接口join
		if len(currentClusterStatus.NodesSSH) == 0 {
			currentClusterStatus.NodesSSH = make(map[string]kubessh.LoginInfo, 0)
		}
		currentClusterStatus.NodesSSH[ip] = nodeSSH
	}

	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return nil
}
