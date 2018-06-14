package cluster

import (
	//"github.com/pkg/errors"
	//"kubeinstall/src/config"
	"kubeinstall/src/logdebug"
	//"kubeinstall/src/runconf"
	//"kubeinstall/src/state"
	//"github.com/pkg/errors"
	"kubeinstall/src/cluster/dnsmonitor"
	"kubeinstall/src/cluster/heapster"
	"kubeinstall/src/cluster/helm"
	"kubeinstall/src/cluster/influxdb"
	"kubeinstall/src/cluster/prometheus"
	"kubeinstall/src/cluster/redis"
	"time"
)

const (
	k8sAppHeapster   = "heapster"
	k8sAppInfluxdb   = "influxdb"
	k8sAppDNSMonitor = "dns-monitor"
	k8sAppPrometheus = "prometheus"
	k8sAppRedis      = "redis"
	k8sAppHelm       = "helm"
)

//AppCfg k8s app 服务定制参数
type AppCfg struct {
	Cpu          int                  `json:"cpu"`
	Mem          int                  `json:"mem"`
	Namespace    string               `json:"namespace"`
	ReplicaCount int                  `json:"replicaCount"`
	Timeout      time.Duration        `json:"timeout"` //单位秒 (int64)
	EmailCfg     prometheus.EMailConf `json:"emailCfg"`
}

//K8sModConf 安装ceph所需的用户定制参数
type K8sModConf struct {
	AppSet map[string]AppCfg `json:"appSet"` //包含了所有APP的配置
}

type callback func(sessionID int, cfg AppCfg)

const (
	defaultCpuCounts = 2
	defaultMemory    = 2048
)

//CheckArgs 参数检查
func (c *AppCfg) checkArgs() bool {
	//if c.Namespace == "" {
	//	return false
	//}

	if c.Cpu == 0 {
		c.Cpu = defaultCpuCounts
	}

	if c.Mem == 0 {
		c.Mem = defaultMemory
	}

	return true
}

func installHeapster(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装heapster----")

	ok := cfg.checkArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---参数校验未通过----")

		return
	}

	c := heapster.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
	}

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	c.PushDockerImage(currentClusterStatus.DockerRegistryURL)

	c.ModifyYaml(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)

	c.Build(anyMasterSSH)

	return
}

func installInfluxdb(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装influxdb----")

	ok := cfg.checkArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---参数校验未通过----")

		return
	}

	c := influxdb.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
	}

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	c.PushDockerImage(currentClusterStatus.DockerRegistryURL)

	c.ModifyYaml(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)

	c.Build(anyMasterSSH)

	return
}

func installDNSMonitor(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装DNS monitor----")

	ok := cfg.checkArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---参数校验未通过----")

		return
	}

	c := dnsmonitor.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
	}

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	c.PushDockerImage(currentClusterStatus.DockerRegistryURL)

	c.ModifyYaml(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)

	c.Build(anyMasterSSH)

	return
}

func installPrometheus(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装Prometheus----")

	ok := cfg.checkArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---参数校验未通过----")

		return
	}

	c := prometheus.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
		EmailCfg:     cfg.EmailCfg,
	}

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	//_, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	ok = c.CheckEmailArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---email参数校验未通过----")

		return
	}

	c.CreateStorage(currentClusterStatus.CephMonNodes, currentClusterStatus.CephNodesSSH)

	c.PushDockerImage(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)

	c.ModifyYaml(
		currentClusterStatus.DockerRegistryURL,
		currentClusterStatus.ProxyAPIServerURL,
		currentClusterStatus.CephMonNodes,
	)

	c.Build(anyMasterSSH)

	return
}

func installRedis(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装Redis----")

	ok := cfg.checkArgs()
	if !ok {
		logdebug.Println(logdebug.LevelError, "---参数校验未通过----")

		return
	}

	c := redis.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
	}

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	c.PushDockerImage(currentClusterStatus.DockerRegistryURL)

	c.ModifyYaml(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)

	c.Build(anyMasterSSH)

	return
}

func installHelm(sessionID int, cfg AppCfg) {
	logdebug.Println(logdebug.LevelInfo, "-----安装helm----")

	c := helm.Creator{
		SessionID:    sessionID,
		Cpu:          cfg.Cpu,
		Mem:          cfg.Mem,
		Namespace:    cfg.Namespace,
		ReplicaCount: cfg.ReplicaCount,
		Timeout:      cfg.Timeout,
	}

	c = c

	if len(currentClusterStatus.Masters) == 0 {
		logdebug.Println(logdebug.LevelError, "---未保存master IP信息---")

		return
	}

	//anyMasterSSH, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	_, ok := currentClusterStatus.NodesSSH[currentClusterStatus.Masters[0]]
	if !ok {
		logdebug.Println(logdebug.LevelError, "---未保存masterSSH信息---")

		return
	}

	//helm安装包解压

	//c.PushDockerImage(currentClusterStatus.DockerRegistryURL)
	//
	//c.ModifyYaml(currentClusterStatus.DockerRegistryURL, currentClusterStatus.ProxyAPIServerURL)
	//
	//c.Build(anyMasterSSH)

	return
}

var execFuncSet = map[string]callback{
	k8sAppHeapster:   installHeapster,
	k8sAppInfluxdb:   installInfluxdb,
	k8sAppDNSMonitor: installDNSMonitor,
	k8sAppPrometheus: installPrometheus,
	k8sAppRedis:      installRedis,
	k8sAppHelm:       installHelm,
}

//InstallK8sModules 安装k8s相关组件
func (p *InstallPlan) InstallK8sModules(sessionID int) error {
	logdebug.Println(logdebug.LevelInfo, "---安装k8s相关组件---")

	for appName, appCfg := range p.K8sModCfg.AppSet {
		execFunc := execFuncSet[appName]
		execFunc(sessionID, appCfg) //同步安装各个软件 (顺序依据map遍历顺序)
	}

	return nil
}
