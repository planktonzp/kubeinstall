package cluster

import (
	"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	"kubeinstall/src/node"
	"kubeinstall/src/runconf"
	"kubeinstall/src/state"
	//"os"
	"kubeinstall/src/session"
	"strings"
	"sync"
)

//NodesConf 节点相关配置
type NodesConf struct {
	NodesIPSet   map[string]kubessh.LoginInfo `json:"nodesIPSet"`
	NodesNameMap map[string]string            `json:"nodesNameMap"`
	DockerGraphs map[string]string            `json:"dockerGraphs"` //key--->ip; value--->graph
}

//var sessionID int

func dynamicModifyNodeJoinCMD(cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	//token := currentClusterStatus.Token //应实时获取
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "token获取失败-master节点未找到")
		return cmdList
	}

	//使master节点执行获取token命令
	k8sccURL := "http://" + currentClusterStatus.Masters[0] + ":" + config.GetWorkerPort() + workerServerPath
	token := getK8sToken(session.InvaildSessionID, k8sccURL, conf.GetK8sTokenCMD)
	if token == "" {
		logdebug.Println(logdebug.LevelError, "token获取失败")

		return cmdList
	}

	masterURL := currentClusterStatus.APIServerURL
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range cmdList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(TOKEN)", token, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(MASTER_API_URL)", masterURL, -1)
		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

//下载ceph配置文件
func downloadCephConf() (string, string) {
	if len(currentClusterStatus.CephMonNodes) == 0 {
		logdebug.Println(logdebug.LevelError, "----ceph未安装 无法下载ceph配置----")

		return "", ""
	}

	//cephConfTempDir := config.GetDestReceiverPath() + "/ceph_tmp/"
	cephConfTempDir := config.GetDownloadsDir()

	monIP := currentClusterStatus.CephMonNodes[0]
	monSSH := currentClusterStatus.CephNodesSSH[monIP]
	cephConf := cephConfTempDir + cephConfName
	cephKey := cephConfTempDir + cephKeyName

	//os.MkdirAll(cephConfTempDir, os.ModePerm)

	client, err := monSSH.ConnectToHost()
	if err != nil {
		return "", ""
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return "", ""
	}

	defer sftpClient.Close()

	kubessh.SftpDownload(sftpClient, cephConfWorkDir+cephConfName, cephConfTempDir)
	kubessh.SftpDownload(sftpClient, cephConfWorkDir+cephKeyName, cephConfTempDir)

	return cephConf, cephKey
}

//发送ceph配置到指定的主机
func sendCephConf(sessionID int, cephConf, cephKey string, nodeSSH kubessh.LoginInfo) error {
	sshClient, err := nodeSSH.ConnectToHost()
	if err != nil {
		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	//从本机/tmp目录发送2份文件到个个node节点
	//kubessh.SftpSend(sftpClient, cephConf, cephConfWorkDir)
	kubessh.ExecSendFileCMD(sshClient, sftpClient, cephConf, cephConfWorkDir)
	//kubessh.SftpSend(sftpClient, cephKey, cephConfWorkDir)
	kubessh.ExecSendFileCMD(sshClient, sftpClient, cephKey, cephConfWorkDir)

	kubessh.ClientRunCMD(sessionID, sshClient, cmd.ExecInfo{CMDContent: "sudo chmod +r /etc/ceph/ceph.client.admin.keyring"})

	return nil
}

//NodesJoin 使node加入当前集群
func (p *InstallPlan) NodesJoin(sessionID int) error {
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
	conf.NodeJoinCMDList = addSchedulePercent(conf.NodeJoinCMDList, "50%")
	logdebug.Println(logdebug.LevelInfo, "-----修改后的cmd--------", conf.NodeJoinCMDList)
	//下载ceph配置到kubeinstall所在主机/tmp
	cephConf, cephKey := downloadCephConf()

	wg := &sync.WaitGroup{}
	wg.Add(len(p.NodesCfg.NodesIPSet))

	for _, nodeSSH := range p.NodesCfg.NodesIPSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			newNodeScheduler(Step7, nodeSSH.HostAddr, "50%", "正在执行node join")

			k8sccURL := "http://" + nodeSSH.HostAddr + ":" + workerPort + workerServerPath
			_, err = sendCMDToK8scc(sessionID, k8sccURL, conf.NodeJoinCMDList)
			if err != nil {
				return
			}

			err := sendCephConf(sessionID, cephConf, cephKey, nodeSSH)
			if err != nil {
				return
			}

			updateNodeScheduler(nodeSSH.HostAddr, "100%", state.Complete, "正在完成node join")

			//操作全部成功 将node保存至全局nodes列表中
			currentClusterStatus.Nodes = saveNode(currentClusterStatus.Nodes, nodeSSH.HostAddr)

			return
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}

	}

	wg.Wait()

	//保存nodeSSH信息
	for ip, nodeSSH := range p.NodesCfg.NodesIPSet {
		//没有通过nodeJoin 仅通过ex接口join
		if len(currentClusterStatus.NodesSSH) == 0 {
			currentClusterStatus.NodesSSH = make(map[string]kubessh.LoginInfo, 0)
		}
		currentClusterStatus.NodesSSH[ip] = nodeSSH
	}

	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	if finalErr != nil {
		return finalErr
	}

	context.State = state.Complete
	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	return finalErr
}

func (p *InstallPlan) dynamicModifyDeleteNodeCMD(cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, nodeName := range p.NodesCfg.NodesNameMap {
		//nodeName := getNodeName(nodeIP)
		for _, cmdInfo := range cmdList {
			cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(NODE_NAME)", nodeName, -1)
			newCMDList = append(newCMDList, cmdInfo)
		}
	}

	return newCMDList
}

func (p *InstallPlan) kubectlDeleteNodes(sessionID int, cmdList []cmd.ExecInfo) {

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelInfo, "-----没有安装master 无法清理-----", currentClusterStatus.Masters, cmdList)

		return
	}

	cmdList = p.dynamicModifyDeleteNodeCMD(cmdList)

	logdebug.Println(logdebug.LevelInfo, "-----delete node-----", currentClusterStatus.Masters, cmdList)

	url := "http://" + currentClusterStatus.Masters[0] + ":" + config.GetWorkerPort() + workerServerPath

	for _, sshInfo := range p.NodesCfg.NodesIPSet {
		nodeState := node.NodeContext{}
		nodeState.HostIP = sshInfo.HostAddr
		nodeState.Step = Step7
		nodeState.Context.SchedulePercent = "50%"
		nodeState.Context.State = state.Running

		req := msg.Request{
			Type:    msg.POST,
			URL:     "http://localhost:" + config.GetAPIServerPort() + "/nodequery",
			Content: nodeState,
		}

		//logdebug.Println(logdebug.LevelInfo, "----req---", req)

		req.SendRequestByJSON()
	}

	cmdList = addSchedulePercent(cmdList, "50%")
	sendCMDToK8scc(sessionID, url, cmdList)

	return
}

//RemoveNodes 移除部分node 可以反复调用
func (p *InstallPlan) RemoveNodes(moduleSessionID int) (string, error) {
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	p.kubectlDeleteNodes(moduleSessionID, conf.DeleteNodesCMDList)

	multiExecCMD(moduleSessionID, p.NodesCfg.NodesIPSet, conf.CleanNodesCMDList)

	for _, sshInfo := range p.NodesCfg.NodesIPSet {
		nodeState := node.NodeContext{}
		//nodeState.HostIP = sshInfo.HostAddr
		//nodeState.Step = Step7
		nodeState.Context.SchedulePercent = "100%"
		nodeState.Context.State = state.Complete

		req := msg.Request{
			Type:    msg.PUT,
			URL:     "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + sshInfo.HostAddr,
			Content: nodeState,
		}

		req.SendRequestByJSON()

		//移除全局保存的nodes信息
		for k, v := range currentClusterStatus.Nodes {
			if sshInfo.HostAddr == v {
				currentClusterStatus.Nodes = append(currentClusterStatus.Nodes[:k], currentClusterStatus.Nodes[k+1:]...)
				delete(currentClusterStatus.NodesSSH, v)

				break
			}
		}
	}

	currentClusterStatus.NodesSSH = p.NodesCfg.NodesIPSet

	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return "移除已经加入集群的node", nil
}
