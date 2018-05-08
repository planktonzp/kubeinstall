package cluster

import (
	//"kubeinstall/src/config"
	//"kubeinstall/src/logdebug"
	//"encoding/json"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"kubeinstall/src/runconf"
	//"kubeinstall/src/client"
	"fmt"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	//"os"
	"sync"
	//"kubeinstall/src/backup"
	//"kubeinstall/src/backup"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	//"kubeinstall/src/node"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/state"
	"strconv"
	"strings"
)

const (
	dockerRunCMD = "docker run -d -p 5000:5000 -v /opt/registry:/var/lib/registry --restart=always registry:latest"
)

const (
	dockerRegistryTarName       = "registry.tar"
	dockerRegistrySHName        = "install-registry.sh"
	dockerCfgPath               = "/etc/sysconfig/docker"
	dockerRegistryTarSrcPathKey = "$(DOCKER_REGISTRY_TAR_PATH)"
	dockerRegistryURLKey        = "$(DOCKER_REGISTRY_URL)"
)

//DockerConf 构建docker仓库所需的信息
type DockerConf struct {
	RegistryIP            string                 `json:"registryIP"`
	RegistryPath          string                 `json:"registryPath"`	//仓库启动使用的路径
	RegistryPort          string                 `json:"registryPort"`
	UserDockerRegistryURL string                 `json:"userDockerRegistryURL"`
	BaseImgTarPath        string                 `json:"baseImgTarPath"`	//基础镜像压缩包路径
	Storage               map[string]storagePlan `json:"storage"` // key --- docker ip ;value --- storage plan
	Graphs                map[string]string      `json:"graphs"`  //k--->docker ip; v--->docker home path
	UseSwap               bool                   `json:"useSwap"`
}

//var sessionID int

func (p *InstallPlan) checkDocker() (msg.ErrorCode, string) {
	////logdebug.Println(logdebug.LevelInfo, "-------检查docker参数-----", p.DockerCfg)
	//if p.DockerCfg.RegistryIP == "" {
	//	return msg.Success
	//}

	//有ip 检查port 如果是空 则应使用默认的port 未实现 不在检查中做
	if p.DockerCfg.RegistryPort == "" {
		//return msg.DockerUserParamInvaild
	}

	if p.DockerCfg.UserDockerRegistryURL == "" && p.DockerCfg.RegistryIP == "" {
		//用户什么也没配
		return msg.DockerRegistryNotSpecified, "docker仓库配置不能为空"
	}

	//logdebug.Println(logdebug.LevelInfo, "-------检查docker参数-----", p.DockerCfg.RegistryIP, "--master--", p.MasterCfg.K8sMasterIPSet)

	//不建议Docker仓库IP与masterIP集合中有交集
	for _, v := range p.MasterCfg.K8sMasterIPSet {

		if p.DockerCfg.RegistryIP == v {
			return msg.DockerRegistryIPConflict, "docker虚拟IP 与 master虚拟IP冲突"
		}
	}

	return p.checkDockerStorage()

	//继续检查用户的URL是否合法 未实现

	//return msg.Success
}

func dynamicCreateDockerRegistryCMD(Path string, Port string, cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range cmdList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(HOST_PATH)", Path, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(HOST_PORT)", Port, -1)
		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func (p *InstallPlan) selfApplyDockerRegistry(conf kubeWorkerCMDConf) {
	conf.ApplyDockerRegistryCMDList = dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, p.DockerCfg.UserDockerRegistryURL, "", p.DockerCfg.UseSwap)

	for _, v := range conf.ApplyDockerRegistryCMDList {
		v.Exec()
	}

	return
}

/*

//直接解压挂载到仓库内不再上传

func (p *InstallPlan) pushDockerImages() (string, error) {
	//执行docker镜像上传脚本
	shFilePath := config.GetSHFilePath() + dockerRegistrySHName
	imagesTarPath := config.GetDockerImagesTarPath()

	cmdInfo := cmd.ExecInfo{
		CMDContent: "sudo chmod 777 " + shFilePath + " && " + shFilePath + " " + p.DockerCfg.UserDockerRegistryURL + " " + imagesTarPath,
		ErrorTags:  []string{"push docker images failed", "Permission denied"},
	}

	logdebug.Println(logdebug.LevelInfo, "==========cmdInfo", cmdInfo)

	//cmd 3 第一遍有错误 上传失败

	return cmdInfo.Exec()
}
*/

func (p *InstallPlan) BaseImgTarExtract() (string, error) {
	//释放基础镜像包，需要保证仓库目录为空

	cmdInfo := cmd.ExecInfo{
		CMDContent: "mkdir -p " + p.DockerCfg.RegistryPath + " && sudo tar " + "-zxf " + p.DockerCfg.BaseImgTarPath + "/baseimg.tar.gz -C " + p.DockerCfg.RegistryPath,
		ErrorTags:  []string{"push docker images failed", "Permission denied"},
	}

	logdebug.Println(logdebug.LevelInfo, "==========cmdInfo", cmdInfo)

	return cmdInfo.Exec()
}

func (p *InstallPlan) createDefaultDockerRegistry(sessionID int) error {
	context := state.Context{
		State: state.Running,
	}

	//docker仓库ip 之前已经POST过了进度则不需要新增 如果 之前没有 则新加
	if _, ok := p.DockerCfg.Storage[p.DockerCfg.RegistryIP]; !ok {
		newNodeScheduler(Step4, p.MachineSSHSet[p.DockerCfg.RegistryIP].HostAddr, "25%", "正在创建docker仓库")
	}

	if _, ok := p.MachineSSHSet[p.DockerCfg.RegistryIP]; !ok {
		context.State = state.Error
		context.ErrMsg = "非法的ip"
		state.Update(sessionID, context)

		setErrNodeScheduler(p.MachineSSHSet[p.DockerCfg.RegistryIP].HostAddr, "", "非法的IP", "")

		return errors.New("非法的ip")
	}

	dockerRegistryTarSrcPath := config.GetTarSrcPath()

	hostSSH := p.MachineSSHSet[p.DockerCfg.RegistryIP]
	sshClient, err := hostSSH.ConnectToHost()
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}

	defer sftpClient.Close()

	//发送registry.tar给主机
	//err = kubessh.SftpSend(sftpClient, dockerRegistryTarSrcPath+dockerRegistryTarName, receiverOPTPath)
	err = kubessh.ExecSendFileCMD(sshClient, sftpClient, dockerRegistryTarSrcPath+dockerRegistryTarName, receiverOPTPath)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}

	logdebug.Println(logdebug.LevelInfo, "----发送registry.tar给主机-----", hostSSH.HostAddr)

	//释放基础镜像压缩包到registry存储目录

	output, err := p.BaseImgTarExtract()

	if err != nil {
		return err
	}

	context.Stdout = output
	context.SchedulePercent = "50%"

	updateNodeScheduler(hostSSH.HostAddr, "50%", state.Running, "正在释放基础镜像压缩包!")

	if err != nil {
		context.State = state.Error
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "释放压缩包失败!")

		return err
	}

	conf := kubeWorkerCMDConf{}
	err = runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}

	conf.CreateDockerRegistryCMDList = dynamicCreateDockerRegistryCMD(p.DockerCfg.RegistryPath, p.DockerCfg.RegistryPort, conf.CreateDockerRegistryCMDList)

	url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	//加载镜像 启动仓库 开始搭建 仓库存在 仅提示 继续往下执行
	//cmd 2
	conf.CreateDockerRegistryCMDList = addSchedulePercent(conf.CreateDockerRegistryCMDList, "75%")

	updateNodeScheduler(hostSSH.HostAddr, "75%", state.Running, "正在创建docker仓库!")

	_, err = sendCMDToK8scc(sessionID, url, conf.CreateDockerRegistryCMDList)

	//填写仓库URL 下一步应用仓库时 会使用 配置本机docker参数 否则push会找不到仓库
	//p.DockerCfg.UserDockerRegistryURL = p.DockerCfg.RegistryIP + ":" + p.DockerCfg.RegistryPort
	p.DockerCfg.UserDockerRegistryURL = config.GetDockerRegistryVip() + ":" + p.DockerCfg.RegistryPort

	p.selfApplyDockerRegistry(conf)

/*
//因为直接解压挂载到容器内 所以不再push
	output, err := p.pushDockerImages()

	if err != nil {
		return err
	}

	context.Stdout = output
*/

	//内部的exec模块没有写输出到state模块
	state.Update(sessionID, context)

	//使用本地docker仓库的URL 去修改所有主机kubelet的启动参数 使其可以从本地仓库下载镜像
	//applyDockerRegistryCMDList 在此列表中添加sed命令

	currentClusterStatus.DockerRegistryURL = p.DockerCfg.UserDockerRegistryURL
	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return err
}

//动态生成docker相关配置
func dynamicCreateDockerCfg(oldCMDList []cmd.ExecInfo, dockerRegistryURL, graphPath string, useSwap bool) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	u := strconv.FormatBool(useSwap)

	//$(DOCKER_REGISTRY_URL)'\" $(DOCKER_CONF_PATH)
	//sed -i '1a\Environment=\"KUBELET_EXTRA_ARGS=--pod-infra-container-image=10.10.3.39:5000/pause-amd64:3.0\"' 10-kubeadm.conf
	for _, cmd := range oldCMDList {
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(DOCKER_REGISTRY_URL)", dockerRegistryURL, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(DOCKER_CONF_PATH)", dockerCfgPath, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(USE_SWAP)", u, -1)
		if graphPath != "" {
			cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(GRAPH_PATH)", graphPath, -1)
		}

		newCMDList = append(newCMDList, cmd)
	}

	return newCMDList
}

func (p *InstallPlan) setDockerNodeState(hostSSH kubessh.LoginInfo) {
	//仓库节点一定POST过 进度是95%
	if p.DockerCfg.RegistryIP == p.MachineSSHSet[hostSSH.HostAddr].HostAddr {
		updateNodeScheduler(hostSSH.HostAddr, "95%", state.Running, "正在应用docker仓库!")

		return
	}

	//node 节点 经历过storage的select 进度75%
	if _, ok := p.DockerCfg.Storage[p.MachineSSHSet[hostSSH.HostAddr].HostAddr]; ok {
		updateNodeScheduler(hostSSH.HostAddr, "75%", state.Running, "正在应用docker仓库!")

		return
	}

	//node 节点 未选择storage 进度50%
	updateNodeScheduler(hostSSH.HostAddr, "50%", state.Running, "正在应用docker仓库!")

	return
}

//dest:OPTIONS='--selinux-enabled --log-driver=journald --signature-verification=false --selinux-enabled -H tcp://0.0.0.0:28015 -H unix:///var/run/docker.sock --insecure-registry=10.10.3.9:5000'
//src: OPTIONS='--selinux-enabled --log-driver=journald --signature-verification=false'
func (p *InstallPlan) applyDockerRegistry(sessionID int) error {
	var (
		finalErr error
		err      error
		output   string
		wg       = &sync.WaitGroup{}
		//将来要能配置这个字段--graph="/var/lib/docker"
	)

	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//conf.CheckKubeletConfCMD = dynamicCreateDockerCfg(conf.CheckKubeletConfCMD, p.DockerCfg.UserDockerRegistryURL)

	logdebug.Println(logdebug.LevelInfo, "--------conf.ApplyDockerRegistryCMDList----------", conf.ApplyDockerRegistryCMDList)
	wg.Add(len(p.MachineSSHSet))

	//cmd 4 仓库90% node节点25%
	for _, hostSSH := range p.MachineSSHSet {
		go func(hostSSH kubessh.LoginInfo) {
			defer wg.Done()

			dockerHomeDir, ok := p.DockerCfg.Graphs[hostSSH.HostAddr]
			if !ok {
				//等待界面完成此字段
				//dockerHomeDir = "/var/lib/docker"
				dockerHomeDir = config.GetDockerHomeDir()
			}

			conf.ApplyDockerRegistryCMDList = dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, p.DockerCfg.UserDockerRegistryURL, dockerHomeDir, p.DockerCfg.UseSwap)
			conf.ModifyKubeletCMDList = dynamicCreateDockerCfg(conf.ModifyKubeletCMDList, p.DockerCfg.UserDockerRegistryURL, dockerHomeDir, p.DockerCfg.UseSwap)

			modifyDockerCMD := fmt.Sprintf(`sudo sed -i "/OPTIONS/c OPTIONS='--graph=%s --log-driver=json-file --signature-verification=false -H tcp://0.0.0.0:28015 -H unix:///var/run/docker.sock --insecure-registry=%s --add-registry=%s'" %s`,
				dockerHomeDir,
				p.DockerCfg.UserDockerRegistryURL,
				p.DockerCfg.UserDockerRegistryURL,
				dockerCfgPath)

			logdebug.Println(logdebug.LevelInfo, "应用docker仓库----URL=", modifyDockerCMD)

			//配置docker节点的状态
			p.setDockerNodeState(hostSSH)

			//发送应用docker仓库的CMD集合
			url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			conf.ApplyDockerRegistryCMDList = append(conf.ApplyDockerRegistryCMDList, cmd.ExecInfo{CMDContent: modifyDockerCMD})
			conf.ApplyDockerRegistryCMDList = addSchedulePercent(conf.ApplyDockerRegistryCMDList, "95%")
			_, err = sendCMDToK8scc(sessionID, url, conf.ApplyDockerRegistryCMDList)
			if err != nil {
				return
			}

			//检查之前是不是写过
			conf.CheckKubeletConfCMD.SchedulePercent = "95%"
			conf.CheckKubeletConfCMD.CMDContent = strings.Replace(conf.CheckKubeletConfCMD.CMDContent, "$(DOCKER_REGISTRY_URL)", p.DockerCfg.UserDockerRegistryURL, -1)
			conf.CheckKubeletConfCMD.CMDContent = strings.Replace(conf.CheckKubeletConfCMD.CMDContent, "$(DOCKER_CONF_PATH)", dockerCfgPath, -1)
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{conf.CheckKubeletConfCMD})
			if output != "" {
				logdebug.Println(logdebug.LevelInfo, "------kubelet参数曾经被修改过-----")

			} else {
				//修改kubelet参数
				conf.ModifyKubeletCMDList = addSchedulePercent(conf.ModifyKubeletCMDList, "95%")
				logdebug.Println(logdebug.LevelInfo, "------修改kubelet参数-----")
				_, err = sendCMDToK8scc(sessionID, url, conf.ModifyKubeletCMDList)
			}

			updateNodeScheduler(hostSSH.HostAddr, "100%", state.Complete, "应用docker仓库完成!")

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

//CreateDockerRegistry 创建docker仓库
func (p *InstallPlan) CreateDockerRegistry(sessionID int) error {
	var (
		err     error
		context state.Context
	)

	logdebug.Println(logdebug.LevelInfo, "---创建docker仓库----Graphs=", p.DockerCfg.Graphs, "--useSwap=", p.DockerCfg.UseSwap)

	//选择docker存储方式
	err = p.selectDockerStorage(sessionID)
	if err != nil {
		return err
	}

	currentClusterStatus.DockerStorage = p.DockerCfg.Storage

	if p.DockerCfg.RegistryIP != "" && p.DockerCfg.RegistryPort != "" {
		//创建默认仓库 并填写docker仓库URL字段供其他主机使用
		err = p.createDefaultDockerRegistry(sessionID)
		currentClusterStatus.DockerRegistryHostAddr = p.DockerCfg.RegistryIP
	}

	if err != nil {
		return err
	}

	err = p.applyDockerRegistry(sessionID)
	if err != nil {
		return err
	}

	currentClusterStatus.UseSwap = p.DockerCfg.UseSwap

	context.State = state.Complete
	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	return err
}

func (p *InstallPlan) cleanDockerRegistry(sessionID int, cmdList []cmd.ExecInfo) {
	logdebug.Println(logdebug.LevelInfo, "-----清理默认的docker registry-----", currentClusterStatus.DockerRegistryURL, cmdList)
	//删除/var/ftp/pub/下的所有rpm包 清理createrepo工具 清理vsftp工具
	sshInfo, ok := p.MachineSSHSet[currentClusterStatus.DockerRegistryHostAddr]
	if !ok {
		logdebug.Println(logdebug.LevelInfo, "-----不是默认的docker仓库 无法清理-----", currentClusterStatus.DockerRegistryHostAddr, cmdList)

		return
	}

	client, err := sshInfo.ConnectToHost()
	if err != nil {

		return
	}

	defer client.Close()

	remoteExecCMD(sessionID, client, cmdList)

	return
}

//RemoveDockerRegistry 移除已经安装的docker仓库
func (p *InstallPlan) RemoveDockerRegistry(moduleSessionID int) (string, error) {
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//multiExecCMD(p.MachineSSHSet, conf.CleanDockerClientCMDList)

	p.cleanDockerRegistry(moduleSessionID, conf.CleanDockerRegistryCMDList)

	p.cleanDockerStorage(conf.CleanDockerClientCMDList)

	return "移除docker仓库", nil
}
