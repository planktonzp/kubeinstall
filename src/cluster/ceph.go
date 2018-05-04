package cluster

import (
	"kubeinstall/src/cmd"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	//"strings"
	"github.com/pkg/errors"
	//"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
	"fmt"
	//"github.com/pkg/sftp"
	//"github.com/pkg/sftp"
	//"k8s.io/kubernetes/pkg/registry/rbac/role"
	"kubeinstall/src/config"
	"kubeinstall/src/runconf"
	//"regexp"
	"github.com/pkg/sftp"
	//"golang.org/x/crypto/ssh"
	//"kubeinstall/src/node"
	"kubeinstall/src/state"
	//"os"
	"strings"
	"sync"
)

//ceph 各个角色类型 同一个node可以身兼数职
const (
	cephRoleOSD = "osd" //必须角色 至少有一个
	cephRoleMON = "mon" //必选角色 至少有一个
	cephRoleMDS = "mds" //额外角色 可以没有
	cephRoleRGW = "rgw" //额外角色 可以没有
)

const (
	monNodeListValue = "$(MON_NODE_LIST)"
)

//互信脚本
const (
	cephSSHScriptFileName = "ceph-ssh.sh"
	cephScriptOptDir      = "/opt"
)

//rdb默认格式 1-基础功能 2-支持快照等高级功能
const (
	rbdFormatInvalid = iota
	rbdFormat1
	rbdFormat2
)

//rbd默认特性 1-只支持分层 2-支持分层加锁等特性(内核3.17以上版本)
const (
	rbdFeaturesInvalid = iota
	rbdFeatures1
	rbdFeatures2
)

const (
	//cephConfTempDir = "/tmp" //kubeinstall下载的ceph配置文件都在这里(然后发送给其他节点)
	cephConfWorkDir = "/etc/ceph/"
	cephConfName    = "ceph.conf"
	cephKeyName     = "ceph.client.admin.keyring"
)

//CephRoleAttribute ceph角色属性
type CephRoleAttribute struct {
	OSDMediaType  string            `json:"osdMediaType"`  //OSD介质类型 file or disk
	OSDMediasPath map[string]string `json:"osdMediasPath"` //OSD介质路径 key---data value----journal 成对出现 且一一对应 不能重复
}

//CephRole 角色
type CephRole struct {
	Types     []string          `json:"types"`     //角色类型-可以指定多个角色
	Attribute CephRoleAttribute `json:"attribute"` //角色属性
}

//CephGlobalConf ceph 全局配置
type CephGlobalConf struct {
	IsOsdCrushUpdateOnStart bool  `json:"isOsdCrushUpdateOnStart"` //false  启动OSD时是否更新crush规则
	OsdJournalSize          int64 `json:"osdJournalSize"`          //20480  MB 日志盘大小
	MonPgWarnMaxPerOsd      int64 `json:"monPgWarnMaxPerOsd"`      //1000   个 单个OSD上PG数量最大值
	MaxOpenFiles            int64 `json:"maxOpenFiles"`            //131072 个 打开文件数量最大值
	OsdScrubBeginHour       int64 `json:"osdScrubBeginHour"`       //1 时 副本数据检测起始时间
	OsdScrubEndHour         int64 `json:"osdScrubEndHour"`         //7 时 副本数据检测结束时间
	RBDDefaultFormat        int64 `json:"rbdDefaultFormat"`        //2  rdb默认格式 1-基础功能 2-支持快照等高级功能
	RBDDefaultFeatures      int64 `json:"rbdDefaultFeatures"`      //1  rbd默认特性 1-只支持分层 2-支持分层加锁等特性(内核3.17以上版本)
}

//CephConf 安装ceph所需的用户定制参数
type CephConf struct {
	NodesSSHSet map[string]kubessh.LoginInfo `json:"nodesSSHSet"`    //ceph集群主机列表
	RolesSet    map[string]CephRole          `json:"rolesSet"`       //角色信息集合
	GlobalCfg   CephGlobalConf               `json:"cephGlobalConf"` //ceph 全局配置
	HostnameSet map[string]string            `json:"hostnameSet"`    //key hostname(cd002) value---(ip 用于查找SSH信息)
}

//var sessionID int

func (p *InstallPlan) checkCeph() msg.ErrorCode {
	isOSD := false
	isMON := false

	logdebug.Println(logdebug.LevelInfo, "-------ceph install check-----", p.CephCfg)

	for _, role := range p.CephCfg.RolesSet {
		for _, t := range role.Types {
			if t == cephRoleOSD {
				isOSD = true
			}

			if t == cephRoleMON {
				isMON = true
			}
		}
	}

	//2个角色必须都有
	if !isMON || !isOSD {
		return msg.CephRolesSetError
	}

	return msg.Success
}

//根据node SSH信息 获取其hostname
func (c *CephConf) getNodeHostname(sessionID int, nodeSSH kubessh.LoginInfo) string {
	url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	cmdInfo := cmd.ExecInfo{
		CMDContent:  "hostname",
		CMDHostAddr: nodeSSH.HostAddr,
	}

	output, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
	if err != nil {
		logdebug.Println(logdebug.LevelError, "----get hostname failed----", err.Error())

		return ""
	}

	h := strings.Split(output, "\n")
	hostname := h[0]

	return hostname

	//for k, v := range c.HostnameSet {
	//	if nodeSSH.HostAddr == v {
	//		return k
	//	}
	//}
	//
	//return ""
}

//获取ceph node网段
func (c *CephConf) getCephNodeSubnet() string {
	subnet := ""

	anyNodeIP := c.getAnyNodeIP()
	if anyNodeIP == "" {
		return subnet
	}

	ipValues := strings.Split(anyNodeIP, ".")

	subnet = fmt.Sprintf(`%s.%s.%s.0`, ipValues[0], ipValues[1], ipValues[2])

	return subnet
}

//获取任意node ip(内部10.10.3.xx 非ssh访问ip)
func (c *CephConf) getAnyNodeIP() string {
	for ip := range c.NodesSSHSet {
		return ip
	}

	return ""
}

func (c *CephConf) getAnyNodeSSH() kubessh.LoginInfo {
	for _, nodeSSH := range c.NodesSSHSet {
		return nodeSSH
	}

	return kubessh.LoginInfo{}
}

func (c *CephConf) installCephPackages(sessionID int, installCephCMDList []cmd.ExecInfo) error {
	var err error
	var finalErr error
	wg := &sync.WaitGroup{}

	wg.Add(len(c.NodesSSHSet))

	//所有节点均安装
	installCephCMDList = addSchedulePercent(installCephCMDList, "6%")
	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			newNodeScheduler(Step8, nodeSSH.HostAddr, "6%", "正在安装ceph组件")

			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			installCephCMDList = addCMDHostAddr(installCephCMDList, nodeSSH.HostAddr)
			_, err = sendCMDToK8scc(sessionID, url, installCephCMDList)

			return
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

//获取任意一个mon node ip
func (c *CephConf) getAnyMonNodeIP() string {
	for ip, role := range c.RolesSet {
		for _, roleType := range role.Types {
			if roleType == cephRoleMON {
				return ip
			}
		}
	}

	return ""
}

func isRole(srcTypes []string, role string) bool {
	for _, roleType := range srcTypes {
		if roleType == role {
			return true
		}
	}

	return false
}

func isNodeIn(srcNodes []string, nodeAddr string) bool {
	for _, src := range srcNodes {
		if src == nodeAddr {
			return true
		}
	}

	return false
}

func saveNode(srcNodes []string, nodeAddr string) []string {
	//newNodes := []string{}
	newNodes := srcNodes

	if isNodeIn(srcNodes, nodeAddr) {
		return srcNodes
	}

	newNodes = append(newNodes, nodeAddr)

	return newNodes
}

func (c *CephConf) saveCephSatus() {
	for ip, nodeSSH := range c.NodesSSHSet {
		if isRole(c.RolesSet[ip].Types, cephRoleOSD) {
			currentClusterStatus.CephOsdNodes = saveNode(currentClusterStatus.CephOsdNodes, nodeSSH.HostAddr)
		}

		if isRole(c.RolesSet[ip].Types, cephRoleMON) {
			currentClusterStatus.CephMonNodes = saveNode(currentClusterStatus.CephMonNodes, nodeSSH.HostAddr)
		}

		if isRole(c.RolesSet[ip].Types, cephRoleMDS) {
			currentClusterStatus.CephMdsNodes = saveNode(currentClusterStatus.CephMdsNodes, nodeSSH.HostAddr)
		}

		if isRole(c.RolesSet[ip].Types, cephRoleRGW) {
			currentClusterStatus.CephRgwNodes = saveNode(currentClusterStatus.CephRgwNodes, nodeSSH.HostAddr)
		}
	}

	currentClusterStatus.CephNodesSSH = c.NodesSSHSet

	runconf.Write(runconf.DataTypeCurrentClusterStatus, currentClusterStatus)

	return
}

func (c *CephConf) downLoadCephFiles() (string, string) {
	monIP := c.getAnyMonNodeIP()
	monSSH := c.NodesSSHSet[monIP]
	//cephConfTempDir := config.GetDestReceiverPath() + "/ceph_tmp/"
	cephConfTempDir := config.GetDownloadsDir()
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

func (c *CephConf) setCephClient(sessionID int, conf kubeWorkerCMDConf) error {
	var (
		finalErr error
		err      error
	)

	//去MON上下载ceph.conf和ceph.client.admin.keyring2个文件到本机/tmp目录
	cephConf, cephKey := c.downLoadCephFiles()

	wg := &sync.WaitGroup{}

	wg.Add(len(currentClusterStatus.NodesSSH))

	for _, nodeSSH := range currentClusterStatus.NodesSSH {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			err = sendCephConf(sessionID, cephConf, cephKey, nodeSSH)

			return
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

//InstallCeph 安装ceph
func (p *InstallPlan) InstallCeph(sessionID int) error {
	var (
		conf    kubeWorkerCMDConf
		err     error
		context state.Context
	)

	context.State = state.Error

	errCode := p.checkCeph()
	if errCode != msg.Success {
		errMsg := msg.GetErrMsg(errCode)
		context.ErrMsg = errMsg

		state.Update(sessionID, context)

		return errors.New(errMsg)
	}

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//安装ceph所需的依赖包
	//cmd 1
	err = p.CephCfg.installCephPackages(sessionID, conf.InstallCephCMDList)
	if err != nil {
		return err
	}

	//根据用户配置 设置ceph节点的主机名
	//cmd 2
	//err = p.CephCfg.setHostname(sessionID)
	//if err != nil {
	//	return err
	//}

	//修改/etc/hosts文件
	//cmd 3
	err = p.CephCfg.modifyHosts(sessionID)
	if err != nil {
		return err
	}

	//配置各个ceph节点间互信
	err = p.CephCfg.setCommunicateEachOther(sessionID)
	if err != nil {
		return err
	}

	//时间同步
	//cmd 6
	err = p.CephCfg.timeSync(sessionID)
	if err != nil {
		return err
	}

	//配置最大进程数限制
	//cmd 7
	err = p.CephCfg.setMaxProcLimit(sessionID)
	if err != nil {
		return err
	}

	//配置打开文件最大数量限制
	//cmd 8
	err = p.CephCfg.setMaxfileLimit(sessionID)
	if err != nil {
		return err
	}

	//配置最大线程数限制
	//cmd 9
	err = p.CephCfg.setMaxThreadLimit(sessionID)
	if err != nil {
		return err
	}

	//应用系统配置文件(上边几部配置的)
	//cmd 10
	err = p.CephCfg.applySysCfg(sessionID)
	if err != nil {
		return err
	}

	//设置ceph集群
	err = p.CephCfg.setCephCluster(sessionID, conf)
	if err != nil {
		return err
	}

	err = p.CephCfg.setCephClient(sessionID, conf)
	if err != err {
		return err
	}

	context.State = state.Complete
	context.SchedulePercent = "100%"
	state.Update(sessionID, context)

	p.CephCfg.saveCephSatus()

	return err
}

func (c *CephConf) getCephNodeHostnameList() string {
	cephNodeHostnameList := ""

	for hostname := range c.HostnameSet {
		cephNodeHostnameList += hostname + " "
	}

	return cephNodeHostnameList
}

func (c *CephConf) dynamicModifyRemoveCephCMD(oldCMDList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}
	cephNodeHostnameList := c.getCephNodeHostnameList()

	for _, cmdInfo := range oldCMDList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(CEPH_NODE_LIST)", cephNodeHostnameList, -1)

		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

//RemoveCeph 删除ceph集群
func (p *InstallPlan) RemoveCeph(moduleSessionID int) error {
	var conf kubeWorkerCMDConf

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	newCMDList := p.CephCfg.dynamicModifyRemoveCephCMD(conf.RemoveCephCMDList)

	logdebug.Println(logdebug.LevelInfo, "---修改后的命令----", newCMDList)

	for _, nodeSSH := range p.CephCfg.NodesSSHSet {
		url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		newCMDList = addCMDHostAddr(newCMDList, nodeSSH.HostAddr)

		sendCMDToK8scc(moduleSessionID, url, newCMDList)
	}

	//卸载最后一步的kill可能会出错 但认为操作成功
	context := state.Context{
		State: state.Complete,
	}
	state.Update(moduleSessionID, context)
	//multiExecCMD(p.CephCfg.NodesSSHSet, conf.RemoveCephCMDList)

	//ceph-deploy命令 在哪些节点执行

	return nil
}
