package helm

import (
	//"fmt"
	//"github.com/pkg/sftp"
	"kubeinstall/src/cmd"
	//"kubeinstall/src/config"
	//"kubeinstall/src/kubessh"
	//"kubeinstall/src/logdebug"
	//"kubeinstall/src/msg"
	//"kubeinstall/src/runconf"
	//"strconv"
	//"strings"
	"time"
)

//Creator heapster创建器
type Creator struct {
	SessionID    int           `json:"sessionID"`
	Cpu          int           `json:"cpu"`
	Mem          int           `json:"mem"`
	Namespace    string        `json:"namespace"`
	ReplicaCount int           `json:"replicaCount"`
	Timeout      time.Duration `json:"timeout"` //单位秒 (int64)
}

//OptCMD 安装操作命令
type OptCMD struct {
	DockerCMDList []cmd.ExecInfo `json:"dockerCMDList"`
	YamlCMDList   []cmd.ExecInfo `json:"yamlCMDList"`
	CreateCMDList []cmd.ExecInfo `json:"createCMDList"`
}
