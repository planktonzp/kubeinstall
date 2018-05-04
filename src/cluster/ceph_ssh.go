package cluster

import (
	"kubeinstall/src/cmd"
	"kubeinstall/src/kubessh"
	"kubeinstall/src/logdebug"
	//"kubeinstall/src/msg"
	//"strings"
	//"github.com/pkg/errors"
	//"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
	"fmt"
	//"github.com/pkg/sftp"
	"github.com/pkg/sftp"
	//"k8s.io/kubernetes/pkg/registry/rbac/role"
	"kubeinstall/src/config"
	//"kubeinstall/src/runconf"
	//"strings"
	//"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/state"
	"sync"
)

//形如: root@cd001.novalocal@docker root@cd002.novalocal@docker user@hostname@password
func (c *CephConf) newScriptArgs(sessionID int) string {
	argList := ""

	for _, nodeSSH := range c.NodesSSHSet {
		hostname := c.getNodeHostname(sessionID, nodeSSH)

		//hostname :=
		args := fmt.Sprintf(`%s@%s@%s `, nodeSSH.UserName, hostname, nodeSSH.Password)
		argList += args
	}

	return argList
}

//scp a.sh cd001.novalocal:/opt/
func copyScript2Node(nodeSSH kubessh.LoginInfo) error {
	cephSSHScriptSrcPath := config.GetSHFilePath() + cephSSHScriptFileName
	cephSSHScriptDestPath := cephScriptOptDir

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

	//kubessh.SftpSend(sftpClient, cephSSHScriptSrcPath, cephSSHScriptDestPath)
	kubessh.ExecSendFileCMD(sshClient, sftpClient, cephSSHScriptSrcPath, cephSSHScriptDestPath)

	return nil
}

//ssh cd001.novalocal -t "/opt/ceph-ssh.sh root@cd002.novalocal@docker root@cd003.novalocal@docker root@cd004.novalocal@docker root@cd001.novalocal@docker"
func (c *CephConf) sendExecScriptCMD(sessionID int, anyNodeSSH kubessh.LoginInfo, argList string) error {
	scriptPath := cephScriptOptDir + "/" + cephSSHScriptFileName
	url := "http://" + anyNodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath
	wg := &sync.WaitGroup{}
	var err error
	var finalErr error

	wg.Add(len(c.NodesSSHSet))

	for _, nodeSSH := range c.NodesSSHSet {
		go func(nodeSSH kubessh.LoginInfo) {
			defer wg.Done()

			updateNodeScheduler(nodeSSH.HostAddr, "", state.Running, "正在发送ceph互信脚本!")

			hostname := c.getNodeHostname(sessionID, nodeSSH)
			cmdInfo := cmd.ExecInfo{
				//ssh root@docker01 -tt "chmod 777 ceph.sh && /opt/ceph-ssh.sh root@docker01 root@docker02 ....05
				CMDContent: fmt.Sprintf(`ssh %s@%s -tt "sudo chmod 777 %s && sudo %s %s"`, nodeSSH.UserName, hostname, scriptPath, scriptPath, argList),
				Tips:       "正在执行ceph互信命令!",
			}

			_, err = sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})
		}(nodeSSH)

		if err != nil {
			finalErr = err
		}
	}

	wg.Wait()

	return finalErr
}

func execScript(sessionID int, nodeSSH kubessh.LoginInfo, argList string) error {
	//hostname := getNodeHostname(nodeSSH)
	scriptPath := cephScriptOptDir + "/" + cephSSHScriptFileName
	url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

	cmdInfo := cmd.ExecInfo{
		CMDContent: fmt.Sprintf(`chmod 777 %s && %s %s`, scriptPath, scriptPath, argList),
		EnvSet:     []string{"USER=root"},
		Tips:       "正在执行ceph互信脚本!",
	}

	_, err := sendCMDToK8scc(sessionID, url, []cmd.ExecInfo{cmdInfo})

	//logdebug.Println(logdebug.LevelError, "----执行ceph脚本结果---", output)

	return err
}

func (c *CephConf) setCommunicateEachOther(sessionID int) error {
	var context state.Context

	context.State = state.Error
	//生成脚本参数列表
	argList := c.newScriptArgs(sessionID)

	for _, nodeSSH := range c.NodesSSHSet {
		err := copyScript2Node(nodeSSH)
		if err != nil {
			logdebug.Println(logdebug.LevelError, "----拷贝ceph脚本失败---", err.Error())
			context.ErrMsg = err.Error()
			state.Update(sessionID, context)

			return err
		}
	}

	anyNodeSSH := c.getAnyNodeSSH()
	//exec 一遍 脚本
	//cmd 4
	err := execScript(sessionID, anyNodeSSH, argList)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "----执行ceph脚本失败---", err.Error())

		return err
	}

	//cmd 5
	err = c.sendExecScriptCMD(sessionID, anyNodeSSH, argList)
	if err != nil {
		logdebug.Println(logdebug.LevelError, "----发送执行ceph脚本命令失败---", err.Error())
		return err
	}

	//logdebug.Println(logdebug.LevelInfo, "----argList----", argList)

	return nil
}
