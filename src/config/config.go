package config

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
)

//StartConfig 启动参数
type StartConfig struct {
	APIServerPort  string //apiserver 监听端口
	LogFileMaxSize int64  //日志文件最大限制
	WorkDir        string //kubeinstall工作根目录
	LogDir         string //生成的日志路径
	RunConfDirPath string //运行配置路径
	LogPrintLevel  string //日志打印级别
	// TODO: 只需要安装yum源的rpm，yum源里面的rpm在tar包里面，在TarSrcPath下面
	RPMPackagesSrcPath  string //安装k8s集群所需所有rpm包的路径-用于创建集群yum源
	RPMPackagesDestPath string //rpm copy至远端主机目标目录
	TarSrcPath          string //创建docker仓库等所需tar包路径
	//无用了
	//DockerImagesTarPath string //docker镜像tar包所在路径(包含CNI的yaml 因为yaml中指定的image路径要与镜像名字一致 故放在一起)
	//DestReceiverPath    string //目标接收目录tar包 脚本等
	WorkerPort          string //worker程序端口
	SHFilePath          string //相关安装脚本所在路径
	YamlFilePath        string //相关yaml文件所在路径
	K8sVersion          string //安装的K8s版本
	K8sAPIServerPort    string //安装的K8s apiserver端口
	VirtualRouterID     string //临时变量 keepalived要求的不确定是多少 原有83 创建失败
	MasterInterfaceName string //k8s master网卡名字
	//CNIType             string //CNI类型
	EntryPort         int    //entry程序端口
	EntryHeart        int    //entry程序心跳间隔(单位秒)
	K8sProxyPort      int    //k8s 对外服务端口
	DownloadsDir      string //下载目录
	CacheDir          string //缓存目录
	DockerRegistryVip string //docker仓库虚IP 手动创建出来之后引入(初始状态是界面选择的那台主机)
	SvcPortRange      string //k8s服务端口范围
	DockerHomeDir     string //docker工作根目录
}

const size1MB = 1024

const (
	defaultAPIServerPort = "9999"
	defaultLogDir        = "/kubeinstall/output/log/"
	defaultRunConfDir    = "/kubeinstall/runconf/"
	defaultLogLevel      = "info"
	defaultLogMaxSize    = 100
	// 存放rpms.tar
	defaultRPMPackagesSrcDir  = "/kubeinstall/rpms/"
	defaultRPMPackagesDestDir = "/var/ftp/pub"
	// 存放 仓库的tar和仓库image的tar
	defaultTarPackagesSrcDir = "/kubeinstall/tars/"
	//defaultDestReceiverPath  = "/opt"
	defaultWorkerPort = "12306"
	defaultSHPath     = "/kubeinstall/sh/"
	defaultYamlPath   = "/kubeinstall/yaml/"
	// 没用了
	//defaultDockerImagesPath    = "/kubeinstall/dockerimages"
	defaultK8sVersion          = "v1.6.4"
	defaultK8sPort             = "6443"
	defaultVirtualRouterID     = "99"
	defaultMasterInterfaceName = "eth0"
	//defaulCNIType              = "flannel"
	defaultEntryPort    = 8011
	defaultEntryHeart   = 15
	defaultK8sProxyPort = 23333
	// 存放需要发送给work的临时目录
	defaultDownloadsDir = "/kubeinstall/output/downloads/"
	//
	defaultCacheDir          = "/kubeinstall/output/cache/"
	defaultDockerRegistryVip = "10.10.3.111"
	defaultSvcPortRange      = "30020-30500"
	defaultDockerHomeDir     = "/var/lib/docker"
)

var startCfg StartConfig
var v bool

func getDefalutWorkDir() (defaultWorkDir string) {
	currentUser, _ := user.Current()

	defaultWorkDir = currentUser.HomeDir

	return
}

//PrintStartConfig 显示启动参数
func PrintStartConfig() {
	log.Println("\n\n\n")

	log.Println("[========================kubeinstall 启动成功========================]")
	log.Println("| [Api server port]:     ", startCfg.APIServerPort)
	log.Println("| [Log debug level]:     ", startCfg.LogPrintLevel)
	log.Println("| [Log dir path]:        ", startCfg.LogDir)
	log.Println("| [Work dir path]:       ", startCfg.WorkDir)
	log.Println("| [Log file max size]:   ", startCfg.LogFileMaxSize/1024)
	log.Println("| [RPMs src path]:       ", startCfg.RPMPackagesSrcPath)
	log.Println("| [RPMs dest path]:      ", startCfg.RPMPackagesDestPath)
	log.Println("| [Dokcer reg tar path]: ", startCfg.TarSrcPath)
	log.Println("| [Worker port]:         ", startCfg.WorkerPort)
	//log.Println("| [Dest recv path]:      ", startCfg.DestReceiverPath)
	log.Println("| [Run conf path]:       ", startCfg.RunConfDirPath)
	log.Println("| [SH file path]:        ", startCfg.SHFilePath)
	log.Println("| [Yaml file path]:      ", startCfg.YamlFilePath)
	//log.Println("| [Docker images path]:  ", startCfg.DockerImagesTarPath)
	log.Println("| [K8s version]:         ", startCfg.K8sVersion)
	log.Println("| [K8s apiserver port]:  ", startCfg.K8sAPIServerPort)
	log.Println("| [Virtual router id]:   ", startCfg.VirtualRouterID)
	log.Println("| [Master interface]:    ", startCfg.MasterInterfaceName)
	//log.Println("| [CNI type]:            ", startCfg.CNIType)
	log.Println("| [Entry port]:          ", startCfg.EntryPort)
	log.Println("| [Entry heart]:         ", startCfg.EntryHeart)
	log.Println("| [K8s proxy port]:      ", startCfg.K8sProxyPort)
	log.Println("| [Downloads dir]:       ", startCfg.DownloadsDir)
	log.Println("| [Cache dir]:           ", startCfg.CacheDir)
	log.Println("| [Docker vip]:          ", startCfg.DockerRegistryVip)
	log.Println("| [Svc port range]:      ", startCfg.SvcPortRange)
	log.Println("| [Docker home dir]:     ", startCfg.DockerHomeDir)
	log.Println("[===================================================================]\n\n\n")

	return
}

func isDirExisted(dirPath string) bool {
	fi, err := os.Stat(dirPath)

	if err != nil {
		return os.IsExist(err)
	}

	return fi.IsDir()
}

//创建kubeinstall相关组件的dir
func CreateModuleDir(packagesDir string) {
	if isDirExisted(packagesDir) {
		//log.Println("存在dir 无需创建", packagesDir)
		return
	}

	//log.Println("不存在dir -----创建", packagesDir)
	log.Println("mkdir--> : ", packagesDir)

	err := os.MkdirAll(packagesDir, os.ModePerm)

	if err != nil {
		log.Println("mkdir err: ", packagesDir)

		log.Fatal(err)
	}

	return
}

//Init 初始化config模块 解析启动参数
func Init(version, buildtime string) {
	flag.StringVar(&startCfg.APIServerPort, "port", defaultAPIServerPort, "API Server Listen Port!")
	flag.StringVar(&startCfg.WorkDir, "workdir", getDefalutWorkDir(), "Work Dir!")
	flag.StringVar(&startCfg.LogDir, "logdir", defaultLogDir, "Log File Dir!")
	flag.StringVar(&startCfg.RunConfDirPath, "runcfgdir", defaultRunConfDir, "Run conf Data Dir!")
	flag.StringVar(&startCfg.LogPrintLevel, "loglevel", defaultLogLevel, "Log Print Level!")
	flag.Int64Var(&startCfg.LogFileMaxSize, "logsize", defaultLogMaxSize, "Log File Max Size(MB)!")
	flag.StringVar(&startCfg.RPMPackagesSrcPath, "rpmsrcdir", defaultRPMPackagesSrcDir, "RPMs src Path!")
	flag.StringVar(&startCfg.RPMPackagesDestPath, "rpmdestdir", defaultRPMPackagesDestDir, "RPMs dest Path!")
	flag.StringVar(&startCfg.WorkerPort, "workerport", defaultWorkerPort, "Worker Listen Port!")
	flag.StringVar(&startCfg.TarSrcPath, "tarsrcdir", defaultTarPackagesSrcDir, "Tars for k8s Src Path!")
	//flag.StringVar(&startCfg.DestReceiverPath, "destrecvdir", defaultDestReceiverPath, "Dest host recevie path(tar,sh..)!")
	flag.StringVar(&startCfg.SHFilePath, "shdir", defaultSHPath, "SH file path!")
	flag.StringVar(&startCfg.YamlFilePath, "yamldir", defaultYamlPath, "Yaml file path!")
	//flag.StringVar(&startCfg.DockerImagesTarPath, "imagesdir", defaultDockerImagesPath, "Docker images tar file path!")
	flag.StringVar(&startCfg.K8sVersion, "k8sversion", defaultK8sVersion, "The version of k8s which is going to be installed!")
	flag.StringVar(&startCfg.K8sAPIServerPort, "k8sport", defaultK8sPort, "The port of k8s apiserver!")
	flag.StringVar(&startCfg.VirtualRouterID, "vrid", defaultVirtualRouterID, "The virtual router id for keepalived!")
	flag.StringVar(&startCfg.MasterInterfaceName, "interface", defaultMasterInterfaceName, "The interface name of master!")
	//flag.StringVar(&startCfg.CNIType, "cni", defaulCNIType, "The cni type of k8s!")
	flag.IntVar(&startCfg.EntryPort, "entryport", defaultEntryPort, "The port of entry!")
	flag.IntVar(&startCfg.EntryHeart, "entryheart", defaultEntryHeart, "The heart of entry!")
	flag.IntVar(&startCfg.K8sProxyPort, "k8sproxyport", defaultK8sProxyPort, "The proxy port of k8s!")
	flag.StringVar(&startCfg.DownloadsDir, "downloadsdir", defaultDownloadsDir, "The downloads dir of kubeinstall!")
	flag.StringVar(&startCfg.CacheDir, "cachedir", defaultCacheDir, "The cache dir of kubeinstall!")
	flag.StringVar(&startCfg.DockerRegistryVip, "dockervip", defaultDockerRegistryVip, "The vip of docker registry!")
	flag.StringVar(&startCfg.SvcPortRange, "svcportrange", defaultSvcPortRange, "The svc port range of k8s!")
	flag.StringVar(&startCfg.DockerHomeDir, "dockerhomedir", defaultDockerHomeDir, "The graph args of docker!")
	flag.BoolVar(&v, "version", false, "Print the version of kubeinstall")

	flag.Parse()

	if v {
		fmt.Printf("version: %s\n", version)
		fmt.Printf("buildtime: %s\n", buildtime)
		os.Exit(0)
	}

	//用户没有配置 则使用默认路径并创建路径 否则 使用用户配置的路径
	if startCfg.LogDir == defaultLogDir {
		startCfg.LogDir = startCfg.WorkDir + startCfg.LogDir
		CreateModuleDir(startCfg.LogDir)
	}
	if startCfg.RunConfDirPath == defaultRunConfDir {
		startCfg.RunConfDirPath = startCfg.WorkDir + startCfg.RunConfDirPath
		CreateModuleDir(startCfg.RunConfDirPath)
	}

	if startCfg.RPMPackagesSrcPath == defaultRPMPackagesSrcDir {
		startCfg.RPMPackagesSrcPath = startCfg.WorkDir + startCfg.RPMPackagesSrcPath
		CreateModuleDir(startCfg.RPMPackagesSrcPath)
	}

	if startCfg.TarSrcPath == defaultTarPackagesSrcDir {
		startCfg.TarSrcPath = startCfg.WorkDir + startCfg.TarSrcPath
		CreateModuleDir(startCfg.TarSrcPath)
	}

	if startCfg.SHFilePath == defaultSHPath {
		startCfg.SHFilePath = startCfg.WorkDir + startCfg.SHFilePath
		CreateModuleDir(startCfg.SHFilePath)
	}

	if startCfg.YamlFilePath == defaultYamlPath {
		startCfg.YamlFilePath = startCfg.WorkDir + startCfg.YamlFilePath
		CreateModuleDir(startCfg.YamlFilePath)
	}
	//
	//if startCfg.DockerImagesTarPath == defaultDockerImagesPath {
	//	startCfg.DockerImagesTarPath = startCfg.WorkDir + startCfg.DockerImagesTarPath
	//	CreateModuleDir(startCfg.DockerImagesTarPath)
	//}

	if startCfg.DownloadsDir == defaultDownloadsDir {
		log.Println("mkdir download :", defaultDownloadsDir)
		startCfg.DownloadsDir = startCfg.WorkDir + startCfg.DownloadsDir
		CreateModuleDir(startCfg.DownloadsDir)
	}

	if startCfg.CacheDir == defaultCacheDir {
		startCfg.CacheDir = startCfg.WorkDir + startCfg.CacheDir
		CreateModuleDir(startCfg.CacheDir)
	}

	startCfg.LogFileMaxSize = startCfg.LogFileMaxSize * size1MB

	//如果不存在 则创建tar包rpm包的目录

	return
}

//GetMasterInterfaceName get interface name of master
func GetMasterInterfaceName() string {
	return startCfg.MasterInterfaceName
}

//GetLogPrintLevel 获取打印级别
func GetLogPrintLevel() string {
	loglevel := startCfg.LogPrintLevel

	return loglevel
}

//GetLogDir 获取日志文件生成路径
func GetLogDir() string {
	logDir := startCfg.LogDir

	return logDir
}

//GetLogFileMaxSize 获取日志文件最大尺寸
func GetLogFileMaxSize() int64 {
	logMaxSize := startCfg.LogFileMaxSize

	return logMaxSize
}

//GetAPIServerPort 获取APIServer端口
func GetAPIServerPort() string {
	port := startCfg.APIServerPort

	return port
}

//GetRPMPackagesSrcPath 获取k8s集群所需要的所有rpm包的路径(至少存在一套)
func GetRPMPackagesSrcPath() string {
	rpmsDir := startCfg.RPMPackagesSrcPath

	return rpmsDir
}

//GetRPMPackagesDestPath 获取在在远端主机的rpm包存放目录
func GetRPMPackagesDestPath() string {
	return startCfg.RPMPackagesDestPath
}

//GetRunConfDirPath 获取备份数据路径
func GetRunConfDirPath() string {
	bkpDir := startCfg.RunConfDirPath

	return bkpDir
}

//GetWorkerPort 获取worker进程服务端口 暂未实现
func GetWorkerPort() string {
	return startCfg.WorkerPort
}

//GetTarSrcPath 获取tar包的路径
func GetTarSrcPath() string {
	return startCfg.TarSrcPath
}

////GetDestReceiverPath 获取接收端路径
//func GetDestReceiverPath() string {
//	return startCfg.DestReceiverPath
//}

//GetSHFilePath 获取脚本文件所在路径
func GetSHFilePath() string {
	return startCfg.SHFilePath
}

//GetYamlFilePath 获取yaml文件所在路径
func GetYamlFilePath() string {
	return startCfg.YamlFilePath
}

//GetDockerImagesTarPath 获取docker镜像tar包路径 /root/kubeinstall/dockerimages (结尾没有/)
func GetDockerImagesTarPath() string {
	//return startCfg.DockerImagesTarPath
	// TODO: 后面删除，不再有kubeinstall启动influxdb,calico等
	return "/kubeinstall/dockerimages"
}

//GetK8sVersion 获取将要安装的k8s版本号
func GetK8sVersion() string {
	return startCfg.K8sVersion
}

//GetK8sAPIServerPort 获取将要安装的k8s apiserver端口
func GetK8sAPIServerPort() string {
	return startCfg.K8sAPIServerPort
}

func GetVirtualRouterID() string {
	return startCfg.VirtualRouterID
}

//GetLocalHostIPV4Addr 获取本机IPV4地址
func GetLocalHostIPV4Addr() string {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		fmt.Println(err)

		return ""
	}

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				//fmt.Println(ipnet.IP.String())
				//获取第一个有效的IP即可
				return ipnet.IP.String()
			}

		}
	}

	return ""
}

/*
//不再获取CNI类型
func GetCNIType() string {
	return startCfg.CNIType
}
*/

//GetEntryPort 获取entry使用的port(在界面未配置的场景下使用)
func GetEntryPort() int {
	return startCfg.EntryPort
}

//GetEntryHeart 获取entry心跳间隔(在界面未配置的场景下使用)
func GetEntryHeart() int {
	return startCfg.EntryHeart
}

//GetK8sProxyPort 获取k8s代理后的端口(在界面未开发此字段的场景下使用)
func GetK8sProxyPort() int {
	return startCfg.K8sProxyPort
}

//GetWorkDir 获取工作目录 /home/paas
func GetWorkDir() string {
	return startCfg.WorkDir
}

//GetDownloadsDir 获取下载目录
func GetDownloadsDir() string {
	return startCfg.DownloadsDir
}

//GetCacheDir 获取缓存目录
func GetCacheDir() string {
	return startCfg.CacheDir
}

//GetDockerRegistryVip 获取docker仓库虚拟IP
func GetDockerRegistryVip() string {
	return startCfg.DockerRegistryVip
}

//GetSvcPortRange 获取服务端口范围
func GetSvcPortRange() string {
	return startCfg.SvcPortRange
}

//GetDockerHomeDir 获取docker工作目录
func GetDockerHomeDir() string {
	return startCfg.DockerHomeDir
}
