package cluster

import (
	"fmt"
	"kubeinstall/src/cmd"
	"kubeinstall/src/config"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"
	//"kubeinstall/src/node"
	"kubeinstall/src/runconf"
	//"kubeinstall/src/state"
	"strconv"
	"strings"
)

const (
	storageTypeThinPool   = "thin_pool" //默认Loop方式
	storageTypeDerictPool = "derict_pool"
)

//thin_pool默认值
const (
	defaultDataVolumePercent     = 95
	defaultMetadataVolumePercnet = 1
	defaultLVMVolumeStorageSize  = 512
	defaultAutoExtendThreshold   = 80
	defaultAutoExtendPercent     = 20
)

//自动扩容配置
type autoExtendConf struct {
	Threshold int `json:"threshold"` //80 自动扩容临界值
	Percent   int `json:"percent"`   //20 自动扩容百分比
}

//thin_pool 配置
type thinPoolConf struct {
	DataVolumePercent     int            `json:"dataVolumePercent"`     //数据卷百分比: 小于等于95 单位%
	MetadataVolumePercent int            `json:"metadataVolumePercent"` //元数据卷百分比: 小于5 单位%
	LVMVolumeStorageSize  int            `json:"lvmVolumeStorageSize"`  //LVM卷存储大小: 256~512 单位K
	AutoExtendCfg         autoExtendConf `json:"autoExtendCfg"`         //自动扩容配置
}

//每个docker节点自己拥有一套storage方案
type storagePlan struct {
	DevPath     []string     `json:"devPath"` //存储设备路径
	VGName      string       `json:"vgName"`  //VG名字
	ThinPoolCfg thinPoolConf `json:"thinPoolCfg"`
}

func (s *storagePlan) dynamicModifyCreateDockerStorageCMD(oldCMDList []cmd.ExecInfo) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}
	devPathSet := ""
	dataVolumePercent := strconv.Itoa(s.ThinPoolCfg.DataVolumePercent)
	metadataVolumePercent := strconv.Itoa(s.ThinPoolCfg.MetadataVolumePercent)
	lvmVolumeStorageSize := strconv.Itoa(s.ThinPoolCfg.LVMVolumeStorageSize)
	thinPoolCfg := fmt.Sprintf(`
activation {
    thin_pool_autoextend_threshold=%d
    thin_pool_autoextend_percent=%d
}
`,
		s.ThinPoolCfg.AutoExtendCfg.Threshold,
		s.ThinPoolCfg.AutoExtendCfg.Percent,
	)

	//组织dev集合
	for _, devPath := range s.DevPath {
		devPathSet += devPath + " "
	}

	for _, cmdInfo := range oldCMDList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(DEV_PATH_SET)", devPathSet, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(VG_NAME)", s.VGName, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(DATA_VOLUME_PERCENT)", dataVolumePercent, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(METADATA_VOLUME_PERCENT)", metadataVolumePercent, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(LVM_VOLUME_STORAGE_SIZE)", lvmVolumeStorageSize, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(THIN_POOL_CFG)", thinPoolCfg, -1)

		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func (p *InstallPlan) selectDockerStorage(sessionID int) error {
	var (
		err  error
		conf kubeWorkerCMDConf
	)

	runconf.Read(runconf.DataTypeKubeWorkerCMDConf, &conf)

	//cmd 1
	for ip, storage := range p.DockerCfg.Storage {
		newNodeScheduler(Step4, p.MachineSSHSet[ip].HostAddr, "25%", "正在选择docker存储!")

		graphPath, ok := p.DockerCfg.Graphs[ip]
		if !ok {
			//等待界面完成此字段
			//dockerHomeDir = "/var/lib/docker"
			graphPath = config.GetDockerHomeDir()
		}

		dockerCfgCMDList := dynamicCreateDockerCfg(conf.ApplyDockerRegistryCMDList, p.DockerCfg.UserDockerRegistryURL, graphPath, p.DockerCfg.UseSwap)

		newCMDList := storage.dynamicModifyCreateDockerStorageCMD(conf.CreateDockerStorageCMDList)
		newCMDList = addSchedulePercent(newCMDList, "25%")
		url := "http://" + p.MachineSSHSet[ip].HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		_, err = sendCMDToK8scc(sessionID, url, dockerCfgCMDList)
		if err != nil {
			return err
		}

		_, err = sendCMDToK8scc(sessionID, url, newCMDList)
		if err != nil {
			return err
		}
	}

	return err
}

func dynamicModifyCleanDockerStorageCMD(oldCMDList []cmd.ExecInfo, s storagePlan) []cmd.ExecInfo {
	newCMDList := []cmd.ExecInfo{}
	devPathSet := ""

	//组织dev集合
	for _, devPath := range s.DevPath {
		devPathSet += devPath + " "
	}

	for _, cmdInfo := range oldCMDList {
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(VG_NAME)", s.VGName, -1)
		cmdInfo.CMDContent = strings.Replace(cmdInfo.CMDContent, "$(DEV_PATH_SET)", devPathSet, -1)

		newCMDList = append(newCMDList, cmdInfo)
	}

	return newCMDList
}

func (p *InstallPlan) cleanDockerStorage(cmdList []cmd.ExecInfo) {
	storage := currentClusterStatus.DockerStorage
	//storage := p.DockerCfg.Storage

	for ip, s := range storage {
		nodeSSH := p.MachineSSHSet[ip]
		url := "http://" + nodeSSH.HostAddr + ":" + config.GetWorkerPort() + workerServerPath

		newCMDList := dynamicModifyCleanDockerStorageCMD(cmdList, s)

		//sendCMDToK8scc(url, newCMDList)
		logdebug.Println(logdebug.LevelInfo, "---修改后的命令--URL--", newCMDList, url)
	}

	return
}

func (p *InstallPlan) checkDockerStorage() (msg.ErrorCode, string) {
	finalErrCode := msg.Success
	finalErrMsg := ""

	for k, s := range p.DockerCfg.Storage {
		if s.VGName == "" {
			finalErrMsg += fmt.Sprintf(`主机[%s]:vgName不能为空
`,
				k,
			)
			logdebug.Println(logdebug.LevelError, "----vgName不能为空----")

			finalErrCode = msg.DockerStorageParamInvalid
		}

		if s.ThinPoolCfg.AutoExtendCfg.Percent == 0 {
			logdebug.Println(logdebug.LevelError, "----ThinPoolCfg.AutoExtendCfg.Percent不能为空----")
			finalErrMsg += fmt.Sprintf(`主机[%s]:自动扩容百分比不能为0
`,
				k,
			)

			finalErrCode = msg.DockerStorageParamInvalid
		}

		if s.ThinPoolCfg.AutoExtendCfg.Threshold == 0 {
			logdebug.Println(logdebug.LevelError, "----ThinPoolCfg.AutoExtendCfg.Threshold不能为空----")
			finalErrMsg += fmt.Sprintf(`主机[%s]:自动扩容临界值不能为0
`,
				k,
			)

			finalErrCode = msg.DockerStorageParamInvalid
		}

		if s.ThinPoolCfg.DataVolumePercent == 0 {
			logdebug.Println(logdebug.LevelError, "----ThinPoolCfg.DataVolumePercent不能为空----")
			finalErrMsg += fmt.Sprintf(`主机[%s]:数据卷百分比不能为0
`,
				k,
			)

			finalErrCode = msg.DockerStorageParamInvalid
		}

		if s.ThinPoolCfg.LVMVolumeStorageSize == 0 {
			logdebug.Println(logdebug.LevelError, "----ThinPoolCfg.LVMVolumeStorageSize不能为空----")
			finalErrMsg += fmt.Sprintf(`主机[%s]:LVM卷存储大小不能为0
`,
				k,
			)
			finalErrCode = msg.DockerStorageParamInvalid
		}

		if s.ThinPoolCfg.MetadataVolumePercent == 0 {
			logdebug.Println(logdebug.LevelError, "----ThinPoolCfg.MetadataVolumePercent不能为空----")
			finalErrMsg += fmt.Sprintf(`主机[%s]:元数据卷百分比不能为0
`,
				k,
			)
			finalErrCode = msg.DockerStorageParamInvalid
		}

		for _, v := range s.DevPath {
			if v == "" {
				logdebug.Println(logdebug.LevelError, "----devPath不能为空----")
				finalErrMsg += fmt.Sprintf(`主机[%s]:存储设备路径比不能为空
`,
					k,
				)
				finalErrCode = msg.DockerStorageParamInvalid
			}
		}
	}

	return finalErrCode, finalErrMsg
}
