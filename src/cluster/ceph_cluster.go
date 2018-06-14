package cluster

import (
	"kubeinstall/src/cmd"
	//"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	//"kubeinstall/src/msg"
	//"strings"
	//"github.com/pkg/errors"
	//"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
	"fmt"
	//"github.com/pkg/sftp"
	//"github.com/pkg/sftp"
	//"k8s.io/kubernetes/pkg/registry/rbac/role"
	"kubeinstall/src/config"
	//"kubeinstall/src/runconf"
	"github.com/pkg/sftp"
	"kubeinstall/src/kubessh"
	//"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/state"
	"strings"
	"sync"
)

func (c *CephConf) setCephCluster(sessionID int, conf kubeWorkerCMDConf) error {
	var (
		err   error
		monIP string
	)

	//配置用户下发的全局ceph配置
	//cmd 11
	monIP, err = c.setGlobalCfg(sessionID, conf)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----set global cfg failed!-----", err.Error())

		return err
	}

	//初始化mon节点
	//cmd 12 13
	err = c.initMonNodes(sessionID, conf, monIP)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----init mon nodes failed!-----", err.Error())

		return err
	}

	//添加osd节点
	err = c.addOSDNodes(sessionID, conf)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "-----add osd nodes failed!-----", err.Error())

		return err
	}

	currentClusterStatus.CephDeployIP = monIP

	return nil
}

func (c *CephConf) getMonNodeCounts() int {
	count := 0
	for ip := range c.NodesSSHSet {
		if isRole(c.RolesSet[ip].Types, cephRoleMON) {
			count++
		}
	}

	return count
}

func (c *CephConf) getOSDNodeCounts() int {
	count := 0
	for ip := range c.NodesSSHSet {
		if isRole(c.RolesSet[ip].Types, cephRoleOSD) {
			count++
		}
	}

	return count
}

//初始化mon节点 (在所有mon节点上执行)
func (c *CephConf) initMonNodes(sessionID int, conf kubeWorkerCMDConf, monIP string) error {
	var err error
	var finalErr error
	monSSH := c.NodesSSHSet[monIP]

	//mon create
	updateNodeScheduler(monSSH.HostAddr, "90%", state.Running, "正在初始化Mon节点!")

	url := "http://" + c.NodesSSHSet[monIP].HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	cmdInfo := cmd.ExecInfo{
		CMDContent:      "cd /etc/ceph && ceph-deploy mon create-initial",
		SchedulePercent: "80%",
		Tips:            "正在初始化Mon节点!",
	}

	//改变属主 否则影响下载
	chownCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`chown %s:users /etc/ceph/*`, monSSH.UserName),
	}

	_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo, chownCMD})
	if err != nil {
		logdebug.Println(logdebug.LevelError, "---发送初始化mon节点命令失败---", err.Error())

		return err
	}

	updateNodeScheduler(monSSH.HostAddr, "100%", state.Complete, "完成初始化Mon节点!")

	//recvDir := config.GetWorkDir() + "/kubeinstall"
	recvDir := config.GetDownloadsDir()

	sshClient, err := monSSH.ConnectToHost()
	if err != nil {
		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	defer sftpClient.Close()
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.conf", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.bootstrap-mds.keyring", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.bootstrap-osd.keyring", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.client.admin.keyring", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.bootstrap-mgr.keyring", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.bootstrap-rgw.keyring", recvDir)
	kubessh.SftpDownload(sftpClient, "/etc/ceph/ceph.mon.keyring", recvDir)

	//-------从mon节点拷贝osd节点的配置-----
	wg2 := &sync.WaitGroup{}
	goRoutineCounts := c.getOSDNodeCounts()
	wg2.Add(goRoutineCounts)

	//mon节点上已经都生成了osd所需的配置文件 拷贝给其他osd节点
	//anyMonNodeIP := c.getAnyMonNodeIP()
	url = "http://" + c.NodesSSHSet[monIP].HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	for ip, nodeSSH := range c.NodesSSHSet {
		if !isRole(c.RolesSet[ip].Types, cephRoleOSD) {
			continue
		}

		go func(nodeSSH kubessh.LoginInfo) {
			defer wg2.Done()

			if nodeSSH.HostAddr == monSSH.HostAddr {
				logdebug.Println(logdebug.LevelInfo, "mon不需要自己拷贝自己的配置---")

				return
			}

			sshClient, err := nodeSSH.ConnectToHost()
			if err != nil {
				return
			}

			defer sshClient.Close()

			sftpClient, err := sftp.NewClient(sshClient)
			if err != nil {
				return
			}

			defer sftpClient.Close()

			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.conf", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.bootstrap-mds.keyring", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.bootstrap-osd.keyring", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.client.admin.keyring", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.bootstrap-mgr.keyring", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.bootstrap-rgw.keyring", "/etc/ceph")
			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.mon.keyring", "/etc/ceph")

		}(nodeSSH)

		if err != nil {
			logdebug.Println(logdebug.LevelError, "---向OSD 发送配置失败---", err.Error())

			finalErr = err
		}
	}

	wg2.Wait()

	return finalErr
}

func (c *CephConf) sendChownCMD(sessionID int, nodeIP string, nodeSSH kubessh.LoginInfo) error {
	role := c.RolesSet[nodeIP]

	if !isRole(role.Types, cephRoleOSD) {
		return nil
	}

	//osd 80% mon 100%

	url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	cmdList := []cmd.ExecInfo{}
	for _, journal := range role.Attribute.OSDMediasPath {
		//组织所有需要修改属主的日志盘命令
		//日志盘没有填写 则不执行
		if journal == "" {
			continue
		}

		cmdInfo := cmd.ExecInfo{
			//chown ceph:ceph /dev/sda4
			CMDContent:      fmt.Sprintf(`chown ceph:ceph %s`, journal),
			SchedulePercent: "92%",
		}

		cmdList = append(cmdList, cmdInfo)
	}

	updateNodeScheduler(nodeSSH.HostAddr, "80%", state.Running, "正在更改配置属主")

	_, err := sendCMDToK8scc(sessionID, url, cmdList)

	return err
}

func (c *CephConf) chownJournalMedia(sessionID int) error {
	var err error
	var finalErr error
	wg := &sync.WaitGroup{}

	wg.Add(len(c.NodesSSHSet))
	//cmd 14
	for ip, nodeSSH := range c.NodesSSHSet {
		go func(ip string, nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			err = c.sendChownCMD(sessionID, ip, nodeSSH)

		}(ip, nodeSSH)

		if err != nil {
			logdebug.Println(logdebug.LevelError, "----发送chown joutnal media命令失败---")
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) sendInstallMediaCMD(sessionID int, nodeIP string, nodeSSH kubessh.LoginInfo) error {
	role := c.RolesSet[nodeIP]
	hostname := c.getNodeHostname(sessionID, nodeSSH)

	//osd 95% mon100%
	if !isRole(role.Types, cephRoleOSD) {
		return nil
	}

	url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	cmdList := []cmd.ExecInfo{}
	for data, journal := range role.Attribute.OSDMediasPath {
		activateCMD := cmd.ExecInfo{}

		if journal != "" {
			journal = ":" + journal
			activateCMD = cmd.ExecInfo{
				//ceph-deploy osd activate selfHostname:/dev/sdb:/dev/sda4
				CMDContent:      fmt.Sprintf(`cd /etc/ceph/ && ceph-deploy --overwrite-conf osd activate %s:%s%s`, hostname, data, journal),
				SchedulePercent: "99%",
			}
		}

		cmdInfo := cmd.ExecInfo{
			//ceph-deploy osd prepare selfHostname:/dev/sdb:/dev/sda4
			CMDContent: fmt.Sprintf(`cd /etc/ceph/ && ceph-deploy --overwrite-conf osd create %s:%s%s`, hostname, data, journal),
		}

		cmdList = append(cmdList, cmdInfo)

		cmdList = append(cmdList, activateCMD)
	}

	_, err := sendCMDToK8scc(sessionID, url, cmdList)

	if err != nil {
		return err
	}

	updateNodeScheduler(nodeSSH.HostAddr, "100%", state.Complete, "完成ods添加")

	return err
}

func (c *CephConf) installOSDMedia(sessionID int) error {
	var err error
	var finalErr error
	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	//cmd 15
	for ip, nodeSSH := range c.NodesSSHSet {
		go func(ip string, nodeSSH kubessh.LoginInfo) {
			defer wg.Done()
			err = c.sendInstallMediaCMD(sessionID, ip, nodeSSH)
		}(ip, nodeSSH)

		if err != nil {
			logdebug.Println(logdebug.LevelError, "----发送prepare osd media命令失败---")

			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) addOSDNodes(sessionID int, conf kubeWorkerCMDConf) error {
	var err error

	//修改每个osd节点的日志盘权限
	err = c.chownJournalMedia(sessionID)
	if err != nil {
		return err
	}

	err = c.installOSDMedia(sessionID)

	return err
}

func (c *CephConf) dynamicModifySetGlobalCfgCMDList(sessionID int, oldCMDList []cmd.ExecInfo) []cmd.ExecInfo {
	monList := ""
	newCMDList := []cmd.ExecInfo{}

	//获取所有mon主机
	for ip, role := range c.RolesSet {
		if isRole(role.Types, cephRoleMON) {
			hostname := c.getNodeHostname(sessionID, c.NodesSSHSet[ip])
			monList += " "
			monList += hostname
		}
	}

	for _, cmdInfo := range oldCMDList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, monNodeListValue, monList, -1)

		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func (c *CephConf) setGlobalCfg(sessionID int, conf kubeWorkerCMDConf) (string, error) {
	newCMDList := c.dynamicModifySetGlobalCfgCMDList(sessionID, conf.SetCephGlobalCfgCMDList)
	var finalErr error

	//界面尚未实现
	if c.GlobalCfg.RBDDefaultFormat == 0 {
		c.GlobalCfg.RBDDefaultFormat = rbdFormat2
	}

	if c.GlobalCfg.RBDDefaultFeatures == 0 {
		c.GlobalCfg.RBDDefaultFeatures = rbdFeatures1
	}

	newGlobalCfgContent := fmt.Sprintf(`public_network=%s/24
	#osd_crush_update_on_start = %t
	osd_journal_size = %d
	mon_pg_warn_max_per_osd = %d
	max open files = %d
	osd_scrub_begin_hour= %d
	osd_scrub_end_hour= %d
	rbd_default_format= %d
	rbd_default_features = %d
		`,
		c.getCephNodeSubnet(),
		c.GlobalCfg.IsOsdCrushUpdateOnStart,
		c.GlobalCfg.OsdJournalSize,
		c.GlobalCfg.MonPgWarnMaxPerOsd,
		c.GlobalCfg.MaxOpenFiles,
		c.GlobalCfg.OsdScrubBeginHour,
		c.GlobalCfg.OsdScrubEndHour,
		c.GlobalCfg.RBDDefaultFormat,
		c.GlobalCfg.RBDDefaultFeatures,
	)

	setGlobalConfCMD := fmt.Sprintf(`echo "%s" >>/opt/ceph/ceph.conf`, newGlobalCfgContent)

	newCMDList = append(newCMDList, cmd.ExecInfo{CMDContent: setGlobalConfCMD})
	newCMDList = addSchedulePercent(newCMDList, "73%")

	//new 命令选择的mon节点 会执行create-init(一次)
	logdebug.Println(logdebug.LevelInfo, "------修改后的ceph global cfg-----", newCMDList)

	monIP := c.getAnyMonNodeIP()
	nodeSSH := c.NodesSSHSet[monIP]

	//改变属主 否则影响下载
	chownCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`chown %s:users /opt/ceph/ceph.conf`, nodeSSH.UserName),
	}

	newCMDList = append(newCMDList, chownCMD)

	url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	_, err := sendCMDToK8scc(sessionID, url, newCMDList)
	if err != nil {
		return monIP, err
	}

	//recvDir := config.GetWorkDir() + "/kubeinstall"
	recvDir := config.GetDownloadsDir()

	sshClient, err := nodeSSH.ConnectToHost()
	if err != nil {
		return monIP, err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return monIP, err
	}

	defer sftpClient.Close()

	kubessh.SftpDownload(sftpClient, "/opt/ceph/ceph.conf", recvDir)

	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	//拷贝到各个节点的/etc/ceph目录
	for _, sshInfo := range c.NodesSSHSet {
		go func(sshInfo kubessh.LoginInfo) {
			defer wg.Done()

			if sshInfo.HostAddr == nodeSSH.HostAddr {
				logdebug.Println(logdebug.LevelInfo, "---生成全局配置的mon不需要自己拷贝自己的配置---")

				return
			}

			updateNodeScheduler(nodeSSH.HostAddr, "73%", state.Running, "正在拷贝ceph配置")

			kubessh.ExecSendFileCMD(sshClient, sftpClient, recvDir+"ceph.conf", "/etc/ceph")

		}(sshInfo)
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return monIP, finalErr
}
