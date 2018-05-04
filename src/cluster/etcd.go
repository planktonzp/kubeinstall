package cluster

import (
	"kubeinstall/src/logdebug"
	//"entry/logdebug"
	"fmt"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/msg"
	"kubeinstall/src/runconf"
	"strings"
	//"sync"
	///"golang.org/x/tools/go/gcimporter15/testdata"
	//"kubeinstall/src/node"
	"kubeinstall/src/state"
)

//EtcdConf 搭建etcd所需的配置
type EtcdConf struct {
	Hosts    []string                     `json:"hosts"`
	SSHHosts map[string]kubessh.LoginInfo `json:"sshHosts"`
}

const (
	etcdInstallSHName            = "install-etcd.sh"
	etcdEnvHost1Key              = "$(ETCD_ENV_HOST1_KEY)"
	etcdEnvHost2Key              = "$(ETCD_ENV_HOST2_KEY)"
	etcdEnvHost3Key              = "$(ETCD_ENV_HOST3_KEY)"
	etcdEnvSelfHostKey           = "$(ETCD_ENV_SELF_HOST_KEY)"
	etcdEnvSelfNameKey           = "$(ETCD_ENV_SELF_NAME_KEY)"
	etcdEnvHost1Value            = "$(ETCD_ENV_HOST1_VALUE)"
	etcdEnvHost2Value            = "$(ETCD_ENV_HOST2_VALUE)"
	etcdEnvHost3Value            = "$(ETCD_ENV_HOST3_VALUE)"
	etcdEnvSelfHostValue         = "$(ETCD_ENV_SELF_HOST_VALUE)"
	etcdEnvSelfNameValue         = "$(ETCD_ENV_SELF_NAME_VALUE)"
	etcdEnvImagePreffixKey       = "$(ETCD_IMAGE_PREFFIX_KEY)"
	etcdEnvImagePreffixValue     = "$(ETCD_IMAGE_PREFFIX_VALUE)"
	defaultEtcdClusterNodeCounts = 3
)

//var sessionID int

func (p *InstallPlan) checkEtcd() msg.ErrorCode {
	etcdCount := len(p.EtcdCfg.Hosts)
	//一个节点 直接建议高可用
	if etcdCount == 1 {
		return msg.EtcdNotHighAvailable
	}

	//0 2 3 4 5 6 ...个节点需要排除0 2 4 6 ...等偶数的情况
	if etcdCount%2 == 0 {
		return msg.EtcdCountInvalid
	}

	return msg.Success
}

//为脚本运行命令 提供环境变量
func dynamicCreateEtcdCMDList(selfName string, selfHost string, allHosts []string, cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}
	newEnvSet := []string{}
	dockerRegistryURL := currentClusterStatus.DockerRegistryURL

	//logdebug.Println(logdebug.LevelInfo, "-------动态修改前的etcdCMDList------", cmdList)

	for _, cmdInfo := range cmdList {
		for _, env := range cmdInfo.EnvSet {
			env = strings.Replace(env, etcdEnvHost1Key, "ETCD_HOST_IP_1", -1)
			env = strings.Replace(env, etcdEnvHost1Value, allHosts[0], -1)
			env = strings.Replace(env, etcdEnvHost2Key, "ETCD_HOST_IP_2", -1)
			env = strings.Replace(env, etcdEnvHost2Value, allHosts[1], -1)
			env = strings.Replace(env, etcdEnvHost3Key, "ETCD_HOST_IP_3", -1)
			env = strings.Replace(env, etcdEnvHost3Value, allHosts[2], -1)
			env = strings.Replace(env, etcdEnvSelfHostKey, "ETCD_SELF_HOST_IP", -1)
			env = strings.Replace(env, etcdEnvSelfHostValue, selfHost, -1)
			env = strings.Replace(env, etcdEnvSelfNameKey, "ETCD_SELF_NAME", -1)
			env = strings.Replace(env, etcdEnvSelfNameValue, selfName, -1)
			env = strings.Replace(env, etcdEnvImagePreffixKey, "ETCD_IMAGE_NAME_PREFFIX", -1)
			env = strings.Replace(env, etcdEnvImagePreffixValue, dockerRegistryURL, -1)

			newEnvSet = append(newEnvSet, env)
		}

		cmdInfo.EnvSet = newEnvSet

		newCMDList = append(newCMDList, cmdInfo)
	}

	logdebug.Println(logdebug.LevelInfo, "-------动态修改后的etcdCMDList------", newCMDList)

	return newCMDList
}

func (p *InstallPlan) joinIntoEtcdCluster(sessionID int, selfName string, host string, conf kubeWorkerCMDConf) error {
	var context state.Context

	context.State = state.Error

	//包括生成生成环境变量 传输脚本 执行脚本系列命令
	etcdCMDList := dynamicCreateEtcdCMDList(selfName, host, p.EtcdCfg.Hosts, conf.InstallEtcdCMDList)

	hostSSH, ok := p.EtcdCfg.SSHHosts[host]
	if !ok {
		errMsg := fmt.Sprintf(`etcd集群安装失败 主机%s SSH不可用`, host)
		context.ErrMsg = errMsg
		state.Update(sessionID, context)
		setErrNodeScheduler(host, "", errMsg, "")

		return errors.New(errMsg)
	}

	sshClient, err := hostSSH.ConnectToHost()
	if err != nil {
		logdebug.Println(logdebug.LevelError, "%s ssh 连接失败", hostSSH.HostAddr, err.Error())
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)
		setErrNodeScheduler(host, "", err.Error(), "")

		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "%s sftp 客户端建立失败", hostSSH.HostAddr, err.Error())
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(host, "", err.Error(), "")

		return err
	}

	defer sftpClient.Close()

	etcdSHFilePath := config.GetSHFilePath() + etcdInstallSHName

	//发送registry.tar给主机
	//err = kubessh.SftpSend(sftpClient, etcdSHFilePath, receiverOPTPath)
	err = kubessh.ExecSendFileCMD(sshClient, sftpClient, etcdSHFilePath, receiverOPTPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "%s sftp 客户端发送文件失败", hostSSH.HostAddr, err.Error())
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(host, "", err.Error(), "")

		return err
	}

	url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	etcdCMDList = addSchedulePercent(etcdCMDList, "50%")

	_, err = sendCMDToK8scc(sessionID, url, etcdCMDList)

	updateNodeScheduler(host, "100%", state.Complete, "完成etcd安装脚本")

	return err
}

//InstallEtcd 安装etcd 生成脚本 发送给选定的主机 然后执行
func (p *InstallPlan) InstallEtcd(sessionID int) error {
	var (
		context state.Context
		conf    kubeWorkerCMDConf
	)

	context.State = state.Complete

	if len(p.EtcdCfg.Hosts) == 1 {
		context.SchedulePercent = "100%"
		state.Update(sessionID, context)

		newNodeScheduler(Step5, p.EtcdCfg.SSHHosts[p.EtcdCfg.Hosts[0]].HostAddr, "50%", "正在执行etcd安装脚本")
		updateNodeScheduler(p.EtcdCfg.SSHHosts[p.EtcdCfg.Hosts[0]].HostAddr, "100%", state.Complete, "完成单节点etcd安装")

		return nil
	}

	if len(p.EtcdCfg.Hosts) != defaultEtcdClusterNodeCounts {
		errMsg := fmt.Sprintf(`当前只支持%d个节点的etcd集群`, defaultEtcdClusterNodeCounts)
		context.State = state.Error
		context.ErrMsg = errMsg
		state.Update(sessionID, context)

		return errors.New(errMsg)
	}

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	for k, host := range p.EtcdCfg.Hosts {
		selfName := fmt.Sprintf(`etcd-node-%d`, k+1)
		if _, ok := p.EtcdCfg.SSHHosts[host]; !ok {
			//ssh列表中未提供 则不安装
			continue
		}

		newNodeScheduler(Step5, p.EtcdCfg.SSHHosts[host].HostAddr, "50%", "正在执行etcd安装脚本")

		err := p.joinIntoEtcdCluster(sessionID, selfName, host, conf)
		if err != nil {
			//出错 内部会写入error
			return err
		}
	}

	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	currentClusterStatus.Etcd = p.EtcdCfg.Hosts
	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return nil
}

//RemoveEtcd 卸载已经安装的etcd(集群/非集群) 在machineSSH中提供etcd的3台主机的SSH信息
func (p *InstallPlan) RemoveEtcd(moduleSessionID int) error {
	var conf kubeWorkerCMDConf
	context := state.Context{
		State: state.Complete,
	}

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	for _, nodeSSH := range p.EtcdCfg.SSHHosts {
		url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		_, err := sendCMDToK8scc(moduleSessionID, url, conf.CleanEtcdCMDList)
		if err != nil {
			context.ErrMsg = err.Error()
			context.State = state.Error

			state.Update(moduleSessionID, context)

			return err
		}

		//logdebug.Println(logdebug.LevelInfo, "----output----", output)
	}

	state.Update(moduleSessionID, context)

	//multiExecCMD(p.EtcdCfg.SSHHosts, conf.CleanEtcdCMDList)

	return nil
}
