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
	"kubeinstall/src/kubessh"
	"kubeinstall/src/msg"
	"kubeinstall/src/node"
	"kubeinstall/src/state"
	"strings"
	"sync"
)

func (c *CephConf) commentOtherServerForNtpCfg(sessionID int) error {
	cmdInfo := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`sed -i "s/^server/#server/" /etc/ntp.conf`),
		//SchedulePercent: "40%",
	}
	wg := &sync.WaitGroup{}
	var err error
	var finalErr error

	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(nodeSSH.HostAddr, "", state.Running, "正在修改ntp服务端配置!")

			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})

			return
		}(nodeSSH)
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) addForNtpServer(sessionID int, subnet string, ntpServerIP string) error {
	ntpServerURL := "http://" + ntpServerIP + ":" + config.GetWorkerPort() + workerServerPath

	ntpServerCfgContent := fmt.Sprintf(`
restrict %s mask 255.255.255.0 nomodify notrap
server 127.127.1.0 fudge 127.127.1.0 stratum 8`,
		subnet,
	)
	ntpServerCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`echo "%s" >>/etc/ntp.conf`, ntpServerCfgContent),
	}

	ntpCheckCMD := cmd.ExecInfo{
		CMDContent: "cat /etc/ntp.conf",
	}

	//检查是否已经添加过
	output, err := sendCMDToK8scc(sessionID, ntpServerURL, []cmd.ExecInfo{ntpCheckCMD})
	if strings.Contains(output, ntpServerCfgContent) {
		logdebug.Println(logdebug.LevelInfo, "---已经添加过ntp server的配置-----")

		return err
	}

	//添加配置
	_, err = sendCMDToK8scc(sessionID, ntpServerURL, []cmd.ExecInfo{
		ntpServerCMD,
		{
			CMDContent: "systemctl restart ntpd && systemctl enable ntpd",
		},
	},
	)

	return err
}

func (c *CephConf) addForNtpClient(sessionID int, ntpServerIP string) error {
	ntpClientCfgContent := fmt.Sprintf(`server %s`, ntpServerIP)
	ntpClientCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`echo "%s" >>/etc/ntp.conf`, ntpClientCfgContent),
		Tips:       "正在修改ntp客户端配置!",
	}
	ntpClientCheckCMD := cmd.ExecInfo{
		CMDContent: "cat /etc/ntp.conf",
	}

	for ip, nodeSSH := range c.NodesSSHSet {
		if ip == ntpServerIP {
			continue
		}
		ntpClientURL := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		//检查是否已经添加过了
		output, err := sendCMDToK8scc(sessionID, ntpClientURL, []cmd.ExecInfo{ntpClientCheckCMD})
		if err != nil {
			return err
		}

		if strings.Contains(output, ntpClientCfgContent) {
			logdebug.Println(logdebug.LevelInfo, "---已经添加过ntp client的配置-----")
			continue
		}

		_, err = sendCMDToK8scc(sessionID, ntpClientURL, []cmd.ExecInfo{
			ntpClientCMD,
			{
				CMDContent: "systemctl restart ntpd && systemctl enable ntpd",
			},
		},
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *CephConf) addNewServerForNtpCfg(sessionID int) error {
	subnet := c.getCephNodeSubnet()
	ntpServerIP := c.getAnyNodeIP()

	//logdebug.Println(logdebug.LevelInfo, "-----subnet -----", subnet)

	err := c.addForNtpServer(sessionID, subnet, c.NodesSSHSet[ntpServerIP].HostAddr)
	if err != nil {
		return err
	}

	err = c.addForNtpClient(sessionID, ntpServerIP)

	return err
}

//时间同步
func (c *CephConf) timeSync(sessionID int) error {
	err := c.commentOtherServerForNtpCfg(sessionID)
	if err != nil {
		return err
	}

	err = c.addNewServerForNtpCfg(sessionID)

	return err
}

func (c *CephConf) setMaxProcLimit(sessionID int) error {
	var output string
	var err error
	var finalErr error

	procLimitCfgContent := fmt.Sprintf(`
* soft nproc unlimited
* hard nproc unlimited
`)

	procLimitCMD := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`echo "%s" >>/etc/security/limits.d/20-nproc.conf`, procLimitCfgContent),
		SchedulePercent: "46%",
		Tips:            "正在修改进程配置!",
	}

	procLimitCheckCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`cat /etc/security/limits.d/20-nproc.conf`),
	}

	wg := &sync.WaitGroup{}

	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{procLimitCheckCMD})
			if err != nil {
				return
			}

			if strings.Contains(output, procLimitCfgContent) {
				//已经添加过 就不再添加了
				return
			}

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{procLimitCMD})
		}(nodeSSH)
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) setMaxfileLimit(sessionID int) error {
	var output string
	var err error
	var finalErr error

	fileLimitCfgContent := fmt.Sprintf(`
* soft nofile 131072
* hard nofile 131072
`)

	fileLimitCMD := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`echo "%s" >>/etc/security/limits.conf`, fileLimitCfgContent),
		SchedulePercent: "53%",
		Tips:            "正在修改最大文件数限制!",
	}

	fileLimitCheckCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`cat /etc/security/limits.conf`),
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			wg.Done()
			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{fileLimitCheckCMD})
			if err != nil {
				return
			}
			if strings.Contains(output, fileLimitCfgContent) {
				//已经添加过 就不再添加了
				return
			}

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{fileLimitCMD})
		}(nodeSSH)
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) setMaxThreadLimit(sessionID int) error {
	var output string
	var err error
	var finalErr error
	nodeState := node.NodeContext{}
	req := msg.Request{}

	threadLimitCfgContent := fmt.Sprintf(`kernel.pid_max = 4194303
`)

	threadLimitCMD := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`echo "%s" >>/etc/sysctl.conf`, threadLimitCfgContent),
		SchedulePercent: "60%",
		Tips:            "正在修改线程最大数限制!",
	}

	threadLimitCheckCMD := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`cat /etc/sysctl.conf`),
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()
			nodeState.Context.SchedulePercent = "60%"
			req.Type = msg.PUT
			req.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + nodeSSH.HostAddr
			req.Content = nodeState
			req.SendRequestByJSON()

			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{threadLimitCheckCMD})
			if err != nil {
				return
			}

			if strings.Contains(output, threadLimitCfgContent) {
				//已经添加过 就不再添加了
				return
			}

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{threadLimitCMD})
		}(nodeSSH)
		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) applySysCfg(sessionID int) error {
	var err error
	var finalErr error

	cmdInfo := cmd.ExecInfo{
		CMDContent:      "sysctl -p",
		SchedulePercent: "66%",
		Tips:            "正在应用系统配置!",
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(nodeSSH.HostAddr, "66%", state.Running, "正在应用系统配置")

			url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func (c *CephConf) getHostsList(sessionID int) string {
	hostsList := ""
	//获取主机名与IP映射列表
	for ip, nodeSSH := range c.NodesSSHSet {
		url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
		cmdInfo := cmd.ExecInfo{
			CMDContent: "hostname",
		}

		hostname, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
		if err != nil {
			logdebug.Println(logdebug.LevelError, "---hostname get error----")

			return ""
		}

		hostLine := ip + " " + hostname
		hostsList += hostLine
	}

	return hostsList
	//hostList := ""
	//
	//for hostname, ip := range c.HostnameSet {
	//	hostList += ip + " " + hostname + "\n"
	//}
	//
	//return hostList
}

func (c *CephConf) setHostname(sessionID int) error {
	nodeState := node.NodeContext{}
	req := msg.Request{}
	var err error
	var finalErr error
	wg := &sync.WaitGroup{}

	wg.Add(len(c.NodesSSHSet))

	//所有节点均配置
	for hostname, ip := range c.HostnameSet {
		//nodeState.HostIP = ip
		//nodeState.Context.State = state.Running
		go func(hostname string, ip string) {

			defer wg.Done()

			if hostname == "" {
				return
			}

			nodeState.Context.SchedulePercent = "13%"
			req.Type = msg.PUT
			req.URL = "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + c.NodesSSHSet[ip].HostAddr
			req.Content = nodeState
			req.SendRequestByJSON()

			setHostnameCMDContent := cmd.ExecInfo{
				CMDContent:      fmt.Sprintf(`hostnamectl set-hostname %s`, hostname),
				SchedulePercent: "13%",
			}
			url := "http://" + c.NodesSSHSet[ip].HostAddr + ":" + config.GetWorkerPort() + workerServerPath

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{setHostnameCMDContent})
		}(hostname, ip)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	logdebug.Println(logdebug.LevelInfo, "----ceph用户没有配置hostname----")

	return finalErr
}

func addHostRules(sessionID int, output, hostsList, url string) error {
	hostListArray := strings.Split(hostsList, "\n")

	for _, hostCfg := range hostListArray {
		if hostCfg == "" {
			continue
		}

		if strings.Contains(output, hostCfg) {
			logdebug.Println(logdebug.LevelInfo, "---已经添加了host---", hostCfg)
			continue
		}

		setHostsCMD := cmd.ExecInfo{
			CMDContent: fmt.Sprintf(`echo "%s" >>/etc/hosts`, hostCfg),
			Tips:       "正在配置hosts!",
		}

		_, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{setHostsCMD})
		if err != nil {
			return err
		}
	}

	return nil
}

//此方法 复用 不宜填写进度
func (c *CephConf) modifyHosts(sessionID int) error {
	hostsList := c.getHostsList(sessionID)
	workPort := config.GetWorkerPort()
	wg := &sync.WaitGroup{}
	var err error
	var finalErr error
	var output string

	wg.Add(len(c.NodesSSHSet))

	logdebug.Println(logdebug.LevelInfo, "---hostList---", hostsList)

	checkHostsCMD := cmd.ExecInfo{
		CMDContent:      fmt.Sprintf(`cat /etc/hosts`),
		SchedulePercent: "",
		//SchedulePercent: "20%",
	}

	//所有节点均配置
	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			//此方法 复用 不宜填写进度
			updateNodeScheduler(nodeSSH.HostAddr, "", state.Running, "正在配置hosts文件...")

			url := "http://" + nodeSSH.HostAddr + ":" + workPort + workerServerPath
			output, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{checkHostsCMD})
			if err != nil {
				return
			}

			err = addHostRules(sessionID, output, hostsList, url)
			if err != nil {
				return
			}
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}
