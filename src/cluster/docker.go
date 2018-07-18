package cluster

import (
	//"kubeinstall/src/config"
	//"kubeinstall/src/logdebug"
	//"encoding/json"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"kubeinstall/src/runconf"

	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	//"os"
	"sync"

	"kubeinstall/src/state"
	"strconv"
	"strings"
)

const (
	//dockerRunCMD = "docker run -d -p 5000:5000 -v /opt/registry:/var/lib/registry --restart=always registry:latest"
	targetPath = "/tmp/"
)

const (
	dockerRegistryTarName       = "registry.tar"
	dockerRegistryBaseimageName = "baseimg.tar.gz"
	//dockerRegistrySHName        = "install-registry.sh"
	dockerCfgPath = "/etc/sysconfig/docker"
	//dockerRegistryTarSrcPathKey = "$(DOCKER_REGISTRY_TAR_PATH)"
	//dockerBaseImagesTarPath     = "$(DOCKER_BASEIMAGES_TAR_PATH)"
	//dockerRegistryURLKey        = "$(DOCKER_REGISTRY_URL)"
)

//DockerConf 构建docker仓库所需的信息
type DockerConf struct {
	RegistryIP            string                 `json:"registryIP"`
	RegistryPort          string                 `json:"registryPort"`
	UserDockerRegistryURL string                 `json:"userDockerRegistryURL"`
	BaseImagesPath        string                 `json:"baseImagesPath"`
	RegistryTarPath       string                 `json:"registryTarPath"`
	Storage               map[string]storagePlan `json:"storage"` // key --- docker ip ;value --- storage plan
	Graphs                map[string]string      `json:"graphs"`  //k--->docker ip; v--->docker home path
	UseSwap               bool                   `json:"useSwap"`
}

//rpc远程解压调用
type Params struct {
	Inpath, Outpath string
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

	if p.DockerCfg.BaseImagesPath == "" {
		p.DockerCfg.BaseImagesPath = targetPath
		config.CreateModuleDir(p.DockerCfg.BaseImagesPath) //2018.5.24  创建docker基础镜像包和仓库镜像包目录
	}


	if p.DockerCfg.RegistryTarPath == "" {
		config.CreateModuleDir(p.DockerCfg.RegistryTarPath) //2018.5.24  创建docker基础镜像包和仓库镜像包目录
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

func dynamicCreateDockerRegistryCMD(p *InstallPlan, cmdList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	for _, cmdInfo := range cmdList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(DOCKER_REGISTRY_TAR_PATH)", targetPath, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(DOCKER_BASEIMAGES_TAR_PATH)", p.DockerCfg.RegistryTarPath, -1) //解压在哪就挂载在哪 2018.5.16
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(HOST_PORT)", p.DockerCfg.RegistryPort, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(CONTAINER_PORT)", p.DockerCfg.RegistryPort, -1)
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
}*/

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

	//发送仓库镜像和基础镜像包给主机
	//err = kubessh.SftpSend(sftpClient, dockerRegistryTarSrcPath+dockerRegistryTarName, receiverOPTPath)

	//p.DockerCfg.RegistryTarPath = p.DockerCfg.RegistryTarPath
	err = kubessh.SftpSend(sftpClient, dockerRegistryTarSrcPath+dockerRegistryTarName, targetPath) //2018.5.16
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}
	//p.DockerCfg.BaseImagesPath = p.DockerCfg.BaseImagesPath //在opt目录下创建子目录 2018.5.16
	//大文件传输
	/*
		err = p.SendbaseimgTar()
		if err != nil {
			return err
		}
	*/
	err = kubessh.SftpSend(sftpClient, dockerRegistryTarSrcPath+dockerRegistryBaseimageName, targetPath) //2018.5.16
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}
	logdebug.Println(logdebug.LevelInfo, "----发送registry.tar和baseimg.tar.gz给主机-----", hostSSH.HostAddr)

	/*
		//调用rpc解压
		err = p.UngzipbaseimgTar()
		if err != nil {
			return err
		}
	*/

	conf := kubeWorkerCMDConf{}
	err = runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "")

		return err
	}

	conf.CreateDockerRegistryCMDList = dynamicCreateDockerRegistryCMD(p, conf.CreateDockerRegistryCMDList)
	logdebug.Println(logdebug.LevelInfo, "仓库挂载的地址为:", p.DockerCfg.RegistryTarPath)

	url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	//加载镜像 启动仓库 开始搭建 仓库存在 仅提示 继续往下执行
	//cmd 2
	conf.CreateDockerRegistryCMDList = addSchedulePercent(conf.CreateDockerRegistryCMDList, "90%")

	updateNodeScheduler(hostSSH.HostAddr, "100%", state.Running, "正在创建docker仓库!")

	_, err = sendCMDToK8scc(sessionID, url, conf.CreateDockerRegistryCMDList)

	//填写仓库URL 下一步应用仓库时 会使用 配置本机docker参数 否则push会找不到仓库
	//旧版本加载镜像后push到仓库地址的动作是在kubeinstall上安装docker完成的 新版本是直接把baseimg发送到远端仓库挂载上
	//p.DockerCfg.UserDockerRegistryURL = p.DockerCfg.RegistryIP + ":" + p.DockerCfg.RegistryPort //2018.5.16
	//p.DockerCfg.UserDockerRegistryURL = config.GetDockerRegistryVip() + ":" + p.DockerCfg.RegistryPort

	//p.selfApplyDockerRegistry(conf)
	/*output, err := p.pushDockerImages()
	if err != nil {
		return err
	}

	context.Stdout = output
	context.SchedulePercent = "75%"

	updateNodeScheduler(hostSSH.HostAddr, "75%", state.Running, "正在执行上传镜像脚本!")

	if err != nil {
		context.State = state.Error
		state.Update(sessionID, context)

		setErrNodeScheduler(hostSSH.HostAddr, "", err.Error(), "执行上传镜像脚本失败!")

		return err
	}*/

	//内部的exec模块没有写输出到state模块
	state.Update(sessionID, context)

	//使用本地docker仓库的URL 去修改所有主机kubelet的启动参数 使其可以从本地仓库下载镜像
	//applyDockerRegistryCMDList 在此列表中添加sed命令

	currentClusterStatus.DockerRegistryURL = p.DockerCfg.UserDockerRegistryURL
	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return err
}

//动态生成docker相关配置
func dynamicCreateDockerCfg(oldCMDList []cmd.ExecInfo, dockerRegistryURL string, graphPath string, useSwap bool) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}

	u := strconv.FormatBool(useSwap)

	//$(DOCKER_REGISTRY_URL)'\" $(DOCKER_CONF_PATH)
	//sed -i '1a\Environment=\"KUBELET_EXTRA_ARGS=--pod-infra-container-image=10.10.3.39:5000/pause-amd64:3.0\"' 10-kubeadm.conf
	for _, cmd := range oldCMDList {
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(DOCKER_REGISTRY_URL)", dockerRegistryURL, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(DOCKER_CONF_PATH)", dockerCfgPath, -1)
		cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(USE_SWAP)", u, -1)
		if graphPath != "" {
			cmd.CMDContent = strings.Replace(cmd.CMDContent, "$(DOCKER_GRAPH_PATH)", graphPath, -1)
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
	//初始化仓库配置信息，用于更改docker主配置文件
	p.DockerCfg.UserDockerRegistryURL = p.DockerCfg.RegistryIP + ":" + p.DockerCfg.RegistryPort
	wg.Add(len(p.MachineSSHSet))

	//cmd 4 仓库90% node节点25%
	for _, hostSSH := range p.MachineSSHSet {
		go func(hostSSH kubessh.LoginInfo) {
			defer wg.Done()

			dockerHomeDir, ok := p.DockerCfg.Graphs[hostSSH.HostAddr]
			if !ok {
				dockerHomeDir = config.GetDockerHomeDir()
			}

			modifyDockerCMD := dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, p.DockerCfg.UserDockerRegistryURL, dockerHomeDir, p.DockerCfg.UseSwap)
			logdebug.Println(logdebug.LevelInfo, "应用docker仓库----URL=", modifyDockerCMD)

			conf.ApplyDockerRegistryCMDList = dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, p.DockerCfg.UserDockerRegistryURL, dockerHomeDir, p.DockerCfg.UseSwap)
			conf.ModifyKubeletCMDList = dynamicCreateDockerCfg(conf.ModifyKubeletCMDList, p.DockerCfg.UserDockerRegistryURL, dockerHomeDir, p.DockerCfg.UseSwap)

			//配置docker节点的状态
			p.setDockerNodeState(hostSSH)

			/*
				//不再重复替换
				modifyDockerCMD := fmt.Sprintf(`sudo sed -i "/OPTIONS/c OPTIONS='--graph=%s --log-driver=json-file --signature-verification=false -H tcp://0.0.0.0:28015 -H unix:///var/run/docker.sock --insecure-registry=%s --add-registry=%s'" %s`,
					dockerHomeDir,
					p.DockerCfg.UserDockerRegistryURL,
					p.DockerCfg.UserDockerRegistryURL,
					dockerCfgPath)
				logdebug.Println(logdebug.LevelInfo, "应用docker仓库----URL=", modifyDockerCMD)
			*/

			//发送应用docker仓库的CMD集合
			url := "http://" + hostSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			//conf.ApplyDockerRegistryCMDList = append(conf.ApplyDockerRegistryCMDList, cmd.ExecInfo{CMDContent: modifyDockerCMD})
			//conf.ApplyDockerRegistryCMDList[0].CMDContent = modifyDockerCMD
			conf.ApplyDockerRegistryCMDList = addSchedulePercent(conf.ApplyDockerRegistryCMDList, "75%")
			//_, err = sendCMDToK8scc(sessionID, url, conf.ApplyDockerRegistryCMDList)

			_, err = sendCMDToK8scc(sessionID, url, modifyDockerCMD)
			if err != nil {
				return
			}

			//检查之前是不是写过
			conf.CheckKubeletConfCMD.SchedulePercent = "95%"
			conf.CheckKubeletConfCMD.CMDContent = strings.Replace(conf.CheckKubeletConfCMD.CMDContent, "$(DOCKER_REGISTRY_URL)", p.DockerCfg.UserDockerRegistryURL, -1)
			conf.CheckKubeletConfCMD.CMDContent = strings.Replace(conf.CheckKubeletConfCMD.CMDContent, "$(DOCKER_CONF_PATH)", dockerCfgPath, -1)
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{conf.CheckKubeletConfCMD})
			conf.ModifyKubeletCMDList = addSchedulePercent(conf.ModifyKubeletCMDList, "100%")
			if output != "" {
				logdebug.Println(logdebug.LevelInfo, "------kubelet参数曾经被修改过-----")

			} else {
				//修改kubelet参数
				logdebug.Println(logdebug.LevelInfo, "------修改kubelet参数-----")
				_, err = sendCMDToK8scc(sessionID, url, conf.ModifyKubeletCMDList)
			}

			//updateNodeScheduler(hostSSH.HostAddr, "100%", state.Complete, "应用docker仓库完成!")

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

	//配置docker-storage,不重启
	err = p.selectDockerStorage(sessionID)
	if err != nil {
		logdebug.Println(logdebug.LevelInfo, "docker-storage config ERR:", err)
		return err
	}

	//配置docker主配置文件,此步重启
	err = p.applyDockerRegistry(sessionID)
	logdebug.Println(logdebug.LevelInfo, "docker config ERR:", err)
	if err != nil {
		return err
	}

	currentClusterStatus.DockerStorage = p.DockerCfg.Storage
	currentClusterStatus.DockerRegistryHostAddr = p.DockerCfg.RegistryIP
	currentClusterStatus.UseSwap = p.DockerCfg.UseSwap

	if p.DockerCfg.RegistryIP != "" && p.DockerCfg.RegistryPort != "" {
		//创建默认仓库 并填写docker仓库URL字段供其他主机使用
		err = p.createDefaultDockerRegistry(sessionID)
	}

	if err != nil {
		return err
	}

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

//解压基础镜像tar包
/*
func (p *InstallPlan) UngzipbaseimgTar() error {
	//连接远程rpc服务
	remoteADDR := p.DockerCfg.RegistryIP + ":12305"
	rpc, err := rpc.DialHTTP("tcp", p.DockerCfg.RegistryIP+":12305")
	if err != nil {
		return err
	}
	defer rpc.Close()
	ret := 0
	//调用远程方法
	//注意第三个参数是指针类型
	path := Params{Inpath: targetPath + dockerRegistryBaseimageName, Outpath: p.DockerCfg.RegistryTarPath}

	logdebug.Println(logdebug.LevelInfo, "\n", path.Inpath, "\n", path.Outpath, "\n", remoteADDR)

	logdebug.Println(logdebug.LevelInfo, "正在解压基础镜像压缩包", p.DockerCfg.UserDockerRegistryURL)

	err2 := rpc.Call("GzipRegistry.Ungzip", path, &ret)
	if err2 != nil {
		logdebug.Println(logdebug.LevelInfo, "解压失败:", err2)

		return err2
	}
	logdebug.Println(logdebug.LevelInfo, "解压完成")

	return nil
}
*/

//RemoveDockerRegistry 移除已经安装的docker仓库
func (p *InstallPlan) RemoveDockerRegistry(moduleSessionID int) (string, error) {
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//multiExecCMD(p.MachineSSHSet, conf.CleanDockerClientCMDList)

	p.cleanDockerRegistry(moduleSessionID, conf.CleanDockerRegistryCMDList)

	p.cleanDockerStorage(conf.CleanDockerClientCMDList)

	return "移除docker仓库", nil
}
