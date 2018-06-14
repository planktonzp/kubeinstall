package cluster

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	//"io"
	"kubeinstall/src/config"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/runconf"
	//"os"
	//"os"
	"github.com/pkg/errors"
	//"io/ioutil"
	//"os"
	"kubeinstall/src/cmd"
	"kubeinstall/src/msg"
	"os"
	"sync"
	//"strings"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/state"
	"strings"
)

//YUMConf 构建yum仓库所需字段
type YUMConf struct {
	IP   string   `json:"ip"`
	Repo repoInfo `json:"repo"`
}

//Shell CMD
const (
	createRepoShellCMD = "sudo createrepo ."
	unZipRPMsShellCMD  = "sudo tar -zxvf"
)

const (
	defaultRepoName     = "ftp"
	defaultGPGKeySuffix = "/RPM-GPG-KEY-CentOS-7"
	repoFilePath        = "/home/pass/kubeinstall/Ftp.repo" //应该后续修改成HOMEDIR
	etcYumReposDPath    = "/etc/yum.repos.d/"
)

const (
	//vsftpPubDir        = "/var/ftp/pub/"
	rpmsTarFileName = "rpms.tar"
	installDirKey   = "$(INSTALL_DIR)"
)

const (
	errorTipExecFailed     = "failed"
	errorTipExecFailure    = "failure"
	errorTipExecError      = "error"
	errorTipNotFound       = "command not found"
	errorTipExecErrorUpper = "Error"
)

//yum相关命令配置文件
type yumCMDConf struct {
	//正向操作命令合集
	//CreatePubDir             cmd.ExecInfo   `json:"createPubDir"`
	InstallVsftpdCMDList     []cmd.ExecInfo `json:"installVsftpdCMDList"`
	StartVsftpdCMDList       []cmd.ExecInfo `json:"startVsftpdCMDList"`
	InstallCreateRepoCMDList []cmd.ExecInfo `json:"installCreateRepoCMDList"`
	BackupOldRepos           cmd.ExecInfo   `json:"backupOldRepos"`
	YUMMakeCacheCMDList      []cmd.ExecInfo `json:"yumMakeCacheCMDList"`

	//反操作命令合集
	RemoveYUMStorageCMDList []cmd.ExecInfo `json:"removeYUMStorageCMDList"`
	RemoveRepoCMDList       []cmd.ExecInfo `json:"removeRepoCMDList"`
}

//如果用户自定义了仓库repo 必须保证各个字段的合法性
type repoInfo struct {
	Name     string `json:"name"`
	BaseURL  string `json:"baseurl"`
	Enabled  bool   `json:"enabled"`
	GPGCheck bool   `json:"gpgcheck"`
	GPGKey   string `json:"gpgkey"`
}

//用户提供的yum仓库搭建参数
type yumInstallPlan struct {
	YUMStorageSSHInfo kubessh.LoginInfo   `json:"yumStorageSSHInfo"`
	NodesSSHInfoSet   []kubessh.LoginInfo `json:"nodesSSHInfoSet"`
	IsUserDefined     bool                `json:"isUserDefined"`
	RepoContent       repoInfo            `json:"repoContent"`
}

//var sessionID int

func (r *repoInfo) isEmpty() bool {
	logdebug.Println(logdebug.LevelInfo, "----check repo---r=", r)
	if r.GPGKey == "" {
		return true
	}

	if r.BaseURL == "" {
		return true
	}

	if r.Name == "" {
		return true
	}

	return false
}

//未实现 后续可以完善检查
func (r *repoInfo) check() bool {
	return true
}

func (p *InstallPlan) checkYUM() msg.ErrorCode {
	if p.YUMCfg.IP != "" {
		return msg.Success
	}

	if p.YUMCfg.Repo.isEmpty() {
		return msg.YUMRegistryNotSpecified
	}

	if !p.YUMCfg.Repo.check() {

		return msg.YUMUserParamInvalid
	}

	return msg.Success
}

func getYumStorageConf() (yumCMDConf, error) {
	conf := yumCMDConf{}

	//conf.InstallCreateRepo =
	err := runconf.Read(runconf.DataTypeYumCMDConf, &conf)
	if err != nil {
		logdebug.Println(logdebug.LevelInfo, err)
		return conf, err
	}

	//如果需要嵌入动态的IP端口路径等值 在这里操作....
	//conf

	//logdebug.Println(logdebug.LevelInfo, "----修改后的参数----", conf)

	return conf, err
}

func parseRPMPath(cmdContent string) (string, string, error) {
	//获取rpm包名
	rpmCMD := strings.Split(cmdContent, installDirKey+"/")
	if len(rpmCMD) != 2 {

		return "", "", errors.New("命令解析失败")
	}

	rpmName := rpmCMD[1]

	//logdebug.Println(logdebug.LevelInfo, "----rpm名字----", vsftpRPMName)

	rpmDestPath := config.GetRPMPackagesDestPath()
	rpmSrcPath := config.GetRPMPackagesSrcPath() + rpmName

	return rpmSrcPath, rpmDestPath, nil
}

func installDependRPM(sessionID int, client *ssh.Client, cmdInfo cmd.ExecInfo) error {
	var (
		err error
	)

	context := state.Context{
		State: state.Running,
	}

	rpmSrcPath, rpmDestPath, err := parseRPMPath(cmdInfo.CMDContent)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	sftpcli, err := sftp.NewClient(client)
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}
	defer sftpcli.Close()
	rpmDestPath = "/tmp/"
	//部署时手动拷贝过去
	if err := kubessh.SftpSend(sftpcli, rpmSrcPath, rpmDestPath); err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	//动态生成安装rpm包命令
	cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, installDirKey, rpmDestPath, -1)

	//cmd 3
	_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
	if err != nil {
		return err
	}

	return nil
}

func remoteStartVsftpd(sessionID int, client *ssh.Client, cmdList []cmd.ExecInfo) error {
	var (
		err error
	)

	//cmd 4
	for _, cmdInfo := range cmdList {
		_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return err
}

//远端执行安装rpm包命令
func remoteInstallRPM(sessionID int, client *ssh.Client, cmdList []cmd.ExecInfo) error {
	var (
		err error
	)

	//cmd 5
	for _, cmdInfo := range cmdList {
		//逐步拷贝rpm包 并单步安装
		err = installDependRPM(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return err
}

/*
[ftp]
name=ftp
baseurl=ftp://172.16.71.172/pub
enabled=1
gpgcheck=0
gpgkey=ftp://172.16.71.172/RPM-GPG-KEY-CentOS-7
*/
func (p *InstallPlan) fillDefaultRepo() {
	//构建数据结构 每个节点据此创建repo文件
	p.YUMCfg.Repo.Name = defaultRepoName
	p.YUMCfg.Repo.Enabled = true
	p.YUMCfg.Repo.GPGCheck = false
	p.YUMCfg.Repo.BaseURL = "ftp://" + p.YUMCfg.IP + "/pub"
	p.YUMCfg.Repo.GPGKey = "ftp://" + p.YUMCfg.IP + defaultGPGKeySuffix

	return
}

func createRepoCache(sessionID int, client *ssh.Client) error {
	vsftpPubDir := config.GetRPMPackagesDestPath()
	var errorTips = []string{
		errorTipExecFailed,
		errorTipExecFailure,
		errorTipExecError,
		errorTipNotFound,
		errorTipExecErrorUpper,
	}

	tarRPMsCMD := cmd.ExecInfo{
		CMDContent:      "cd " + vsftpPubDir + " && " + unZipRPMsShellCMD + " " + rpmsTarFileName + " && chmod 777 * -R ",
		ErrorTags:       errorTips,
		SuccessTags:     []string{},
		SchedulePercent: "60%",
		Tips:            "正在解压rpm包",
	}

	//cmd 6
	_, err := kubessh.ClientRunCMD(sessionID, client, tarRPMsCMD)
	if err != nil {
		return err
	}

	createRepoCMD := cmd.ExecInfo{
		CMDContent:      "cd " + vsftpPubDir + " && " + createRepoShellCMD,
		ErrorTags:       errorTips,
		SuccessTags:     []string{},
		SchedulePercent: "70%",
		Tips:            "正在创建repo",
	}

	//cmd 7
	_, err = kubessh.ClientRunCMD(sessionID, client, createRepoCMD)

	return err
}

//创建默认的yum仓库
func (p *InstallPlan) createDefaultYumStorage(sessionID int) error {
	var (
		client *ssh.Client
		err    error
		conf   yumCMDConf
	)

	context := state.Context{}

	conf, err = getYumStorageConf()
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	if _, ok := p.MachineSSHSet[p.YUMCfg.IP]; !ok {
		context.State = state.Error
		context.ErrMsg = "非法的IP"
		state.Update(sessionID, context)

		return err
	}

	yumHost := p.MachineSSHSet[p.YUMCfg.IP]

	client, err = yumHost.ConnectToHost()
	if err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	defer client.Close()

	//备份旧的repo文件-cmd 1
	conf.BackupOldRepos.SchedulePercent = "10%"

	newNodeScheduler(Step2, yumHost.HostAddr, "10%", "正在yum源所在节点备份旧的yum配置")

	_, err = kubessh.ClientRunCMD(sessionID, client, conf.BackupOldRepos)
	if err != nil {
		return err
	}

	//创建本地yum源rpm包存放目录 cmd 2
	//	updateNodeScheduler(yumHost.HostAddr, "20%", state.Running, "正在创建pub目录")
	//	conf.CreatePubDir.SchedulePercent = "20%"
	//
	//	_, err = kubessh.ClientRunCMD(sessionID, client, conf.CreatePubDir)
	//	if err != nil {
	//		return err
	//	}

	//在远端安装rpm包 此命令合集中只能是rpm -ivh xxx系列命令
	conf.InstallVsftpdCMDList = addSchedulePercent(conf.InstallVsftpdCMDList, "30%")
	updateNodeScheduler(yumHost.HostAddr, "30%", state.Running, "正在安装vsftpd")

	err = remoteInstallRPM(sessionID, client, conf.InstallVsftpdCMDList)
	if err != nil {
		return err
	}

	//远端启动vsftpd
	conf.StartVsftpdCMDList = addSchedulePercent(conf.StartVsftpdCMDList, "40%")
	updateNodeScheduler(yumHost.HostAddr, "40%", state.Running, "正在启动vsftpd")

	err = remoteStartVsftpd(sessionID, client, conf.StartVsftpdCMDList)
	if err != nil {
		return err
	}

	//远端安装rpm包
	conf.InstallCreateRepoCMDList = addSchedulePercent(conf.InstallCreateRepoCMDList, "50%")
	updateNodeScheduler(yumHost.HostAddr, "50%", state.Running, "正在安装createrepo软件")

	err = remoteInstallRPM(sessionID, client, conf.InstallCreateRepoCMDList)
	if err != nil {
		return err
	}

	//将本地的rpms scp至 仓库主机/var/ftp/pub/
	rpmsTarSrcDir := config.GetTarSrcPath()
	vsftpPubDir := config.GetRPMPackagesDestPath()

	//部署时手动拷贝过去
	if err := sshSendFile(client, rpmsTarSrcDir+rpmsTarFileName, vsftpPubDir); err != nil {
		context.State = state.Error
		context.ErrMsg = err.Error()
		state.Update(sessionID, context)

		return err
	}

	p.fillDefaultRepo()

	err = createRepoCache(sessionID, client)

	return err
}

func (p *InstallPlan) yumMakeCache(sessionID int, client *ssh.Client, host string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	conf := yumCMDConf{}
	//yum clean all && yum makecache
	runconf.Read(runconf.DataTypeYumCMDConf, &conf)

	conf.BackupOldRepos.CMDHostAddr = host

	//cmd 8 外层一个循环 取最后的一次进度

	//该节点已经操作完成 其他节点头开始
	if p.MachineSSHSet[p.YUMCfg.IP].HostAddr == host {
		updateNodeScheduler(host, "60%", state.Running, "正在备份旧的yum配置!")
	} else {
		newNodeScheduler(Step2, host, "30%", "正在备份旧的yum配置!")
	}

	_, err = kubessh.ClientRunCMD(sessionID, client, conf.BackupOldRepos)
	if err != nil {
		return err
	}

	//repoFilePath := config.GetWorkDir() + "/kubeinstall/Ftp.repo"
	repoFilePath := config.GetDownloadsDir() + "Ftp.repo"
	//kubessh.SftpSend(sftpClient, repoFilePath, etcYumReposDPath)
	err = kubessh.ExecSendFileCMD(client, sftpClient, repoFilePath, etcYumReposDPath)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "发送Ftp.repo至主机:", host, "时失败,原因:", err)
		setErrNodeScheduler(host, "", err.Error(), "发送repo失败!")

		return err
	}
	logdebug.Println(logdebug.LevelInfo, "发送Ftp.repo至主机", host)

	//cmd 9
	if p.YUMCfg.IP == host {
		conf.YUMMakeCacheCMDList = addSchedulePercent(conf.YUMMakeCacheCMDList, "90%")
		updateNodeScheduler(host, "90%", state.Running, "正在执行yum makecache!")
	} else {
		conf.YUMMakeCacheCMDList = addSchedulePercent(conf.YUMMakeCacheCMDList, "60%")
		updateNodeScheduler(host, "60%", state.Running, "正在执行yum makecache!")
	}

	for _, cmdInfo := range conf.YUMMakeCacheCMDList {
		cmdInfo.CMDHostAddr = host
		_, err = kubessh.ClientRunCMD(sessionID, client, cmdInfo)
		if err != nil {
			return err
		}
	}

	return err
}

//在/tmp/目录下构建出Ftp.repo 后续会发送给各个主机
func (p *InstallPlan) buildRepoFile() {
	//repoFilePath := config.GetWorkDir() + "/kubeinstall/Ftp.repo"
	repoFilePath := config.GetDownloadsDir() + "Ftp.repo"

	logdebug.Println(logdebug.LevelInfo, "构建repo文件", repoFilePath)

	os.Remove(repoFilePath)

	file, err := os.Create(repoFilePath)

	if err != nil {
		return
	}

	defer file.Close()

	enable := 0
	gpgCheck := 0

	if p.YUMCfg.Repo.Enabled {
		enable = 1
	}

	if p.YUMCfg.Repo.GPGCheck {
		gpgCheck = 1
	}

	repoContent := fmt.Sprintf(`[ftp]
name=%s
baseurl=%s
enabled=%d
gpgcheck=%d
gpgkey=%s
`,
		p.YUMCfg.Repo.Name,
		p.YUMCfg.Repo.BaseURL,
		enable,
		gpgCheck,
		p.YUMCfg.Repo.GPGKey,
	)

	file.WriteString(repoContent)

	return
}

//应用指定的yum仓库
func (p *InstallPlan) applyYumStorage(sessionID int) error {
	var (
		client   *ssh.Client
		err      error
		finalErr error
	)
	context := state.Context{}

	if p.YUMCfg.Repo.BaseURL == "" || p.YUMCfg.Repo.GPGKey == "" {
		context.State = state.Error
		context.ErrMsg = "apply yum storage failed"
		state.Update(sessionID, context)

		return errors.New("apply yum storage failed")
	}

	p.buildRepoFile()

	wg := &sync.WaitGroup{}
	wg.Add(len(p.MachineSSHSet))
	//mutexLock := &sync.Mutex{}

	//yum仓库安装所在的主机 同样需要makecache
	for _, nodeSSH := range p.MachineSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			client, err = nodeSSH.ConnectToHost()
			if err != nil {
				return
			}

			defer client.Close()

			err = p.yumMakeCache(sessionID, client, nodeSSH.HostAddr)

			//出错后屏幕输入已经更新 记录曾经出错 外层调用
			if err != nil {
				finalErr = err
				setErrNodeScheduler(nodeSSH.HostAddr, "", err.Error(), "yum makecache失败")
			} else {
				updateNodeScheduler(nodeSSH.HostAddr, "100%", state.Complete, "yum源应用完成!")
			}

		}(nodeSSH)

	}

	wg.Wait()

	return finalErr
}

//CreateYUMStorage 创建YUM仓库
func (p *InstallPlan) CreateYUMStorage(sessionID int) {
	var (
		err     error
		context state.Context
	)

	logdebug.Println(logdebug.LevelInfo, "---创建YUM仓库---")

	//使用集群内的某一台主机创建YUM源
	if p.YUMCfg.IP != "" {
		//暂时未使用返回的标准输出
		err = p.createDefaultYumStorage(sessionID)
		currentClusterStatus.YUMHostAddr = p.YUMCfg.IP
	}

	if err != nil {
		//创建默认仓库失败 内部会写入错误结果
		return
	}

	context.State = state.Complete
	context.SchedulePercent = "100%"
	err = p.applyYumStorage(sessionID)
	if err != nil {
		//出错写入结果
		context.State = state.Error
		//出错 不更新进度
		context.SchedulePercent = ""
	}

	state.Update(sessionID, context)

	currentClusterStatus.YUMStorageRepo = p.YUMCfg.Repo
	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return
}

//由于此时不能保证下游节点已经安装了k8scc 所以只能使用ssh方式去清理
func (p *InstallPlan) cleanYUMStorage(sessionID int, cmdList []cmd.ExecInfo) {
	logdebug.Println(logdebug.LevelInfo, "-----清理默认的yum源-----", currentClusterStatus.YUMHostAddr, cmdList)
	//删除/var/ftp/pub/下的所有rpm包 清理createrepo工具 清理vsftp工具
	sshInfo, ok := p.MachineSSHSet[currentClusterStatus.YUMHostAddr]
	if !ok {
		logdebug.Println(logdebug.LevelInfo, "-----不是默认的yum源 无法清理-----", currentClusterStatus.YUMHostAddr, cmdList)

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

//RemoveYUMStorage 移除yum源
func (p *InstallPlan) RemoveYUMStorage(moduleSessionID int) (string, error) {
	var conf yumCMDConf

	runconf.Read(runconf.DataTypeYumCMDConf, &conf)

	//必须保证之前安装过yum源 才能清理
	p.cleanYUMStorage(moduleSessionID, conf.RemoveYUMStorageCMDList)

	multiExecCMD(moduleSessionID, p.MachineSSHSet, conf.RemoveRepoCMDList)
	//p.cleanRepo(conf.RemoveRepoCMDList)

	return "移除yum源", nil
}
