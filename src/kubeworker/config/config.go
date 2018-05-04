package config

import (
	"flag"
	"log"
	"os/user"
	"strings"
)

//StartConfig 启动参数
type StartConfig struct {
	APIServerPort                 string   //apiserver 监听端口
	LogFileMaxSize                int64    //日志文件最大限制
	WorkDir                       string   //kubeworker工作根目录
	LogDir                        string   //生成的日志路径
	BkpDataDirPath                string   //备份数据路径
	LogPrintLevel                 string   //日志打印级别
	FireWallStopCMD               string   //防火墙关闭命令
	ExecCMDErrorCheckPoints       []string //执行服务命令错误检查点
	StopServiceSuccessCheckPoints []string //停止服务命令错误检查点-成功标识
}

const size1MB = 1024

const (
	defaultAPIServerPort                 = "9998"
	defaultLogDir                        = "/kubeworker/log/"
	defaultBkpDir                        = "/kubeworker/bkp/"
	defaultLogLevel                      = "info"
	defaultLogMaxSize                    = 100
	defaultFireWallStopCMD               = "systemctl stop firewalld"
	defaultExecCMDErrCheckPoints         = "failed,failure,error,command not found,Error"
	defaultStopServiceSuccessCheckPoints = "Active: inactive (dead)"
)

var startCfg StartConfig

func getDefalutWorkDir() (defaultWorkDir string) {
	currentUser, _ := user.Current()

	defaultWorkDir = currentUser.HomeDir

	return
}

//PrintStartConfig 显示启动参数
func PrintStartConfig() {
	log.Println("\n\n\n")

	log.Println("[========================Start Conf========================]")
	log.Println("| [Api server port]:     ", startCfg.APIServerPort)
	log.Println("| [Log debug level]:     ", startCfg.LogPrintLevel)
	log.Println("| [Log dir path]:        ", startCfg.LogDir)
	log.Println("| [Work dir path]:       ", startCfg.WorkDir)
	log.Println("| [Log file max size]:   ", startCfg.LogFileMaxSize/1024)
	log.Println("| [Error Check points]:  ", startCfg.ExecCMDErrorCheckPoints)
	log.Println("[==========================================================]\n\n\n")

	return
}

//Init 初始化config模块 解析启动参数
func Init() {
	var errCheckPointsList string
	var successCheckPointsList string

	flag.StringVar(&startCfg.APIServerPort, "port", defaultAPIServerPort, "API Server Listen Port!")
	flag.StringVar(&startCfg.WorkDir, "workdir", getDefalutWorkDir(), "Work Dir!")
	flag.StringVar(&startCfg.LogDir, "logdir", defaultLogDir, "Log File Dir!")
	flag.StringVar(&startCfg.BkpDataDirPath, "bkpdir", defaultBkpDir, "Backup Data Dir!")
	flag.StringVar(&startCfg.LogPrintLevel, "loglevel", defaultLogLevel, "Log Print Level!")
	flag.Int64Var(&startCfg.LogFileMaxSize, "logsize", defaultLogMaxSize, "Log File Max Size(MB)!")
	flag.StringVar(&startCfg.FireWallStopCMD, "fwstopcmd", defaultFireWallStopCMD, "Stop FW CMD!")
	flag.StringVar(&errCheckPointsList, "errtips", defaultExecCMDErrCheckPoints, "Error Check points!")
	flag.StringVar(&successCheckPointsList, "successtips", defaultStopServiceSuccessCheckPoints, "Success Stop a services flag!")

	flag.Parse()

	startCfg.LogDir = startCfg.WorkDir + startCfg.LogDir
	startCfg.BkpDataDirPath = startCfg.WorkDir + startCfg.BkpDataDirPath
	startCfg.LogFileMaxSize = startCfg.LogFileMaxSize * size1MB

	startCfg.ExecCMDErrorCheckPoints = strings.Split(errCheckPointsList, ",")
	startCfg.StopServiceSuccessCheckPoints = strings.Split(successCheckPointsList, ",")

	return
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

//GetBkpDataDirPath 获取备份数据路径
func GetBkpDataDirPath() string {
	bkpDir := startCfg.BkpDataDirPath

	return bkpDir
}

//GetFireWallStopCMD 获取防火墙关闭命令
func GetFireWallStopCMD() string {
	fwCMD := startCfg.FireWallStopCMD

	return fwCMD
}

//GetStopServiceCheckPoints 获取检查点
func GetStopServiceCheckPoints() ([]string, []string) {
	errCheckPoints := startCfg.ExecCMDErrorCheckPoints
	successCheckPoints := startCfg.StopServiceSuccessCheckPoints

	return errCheckPoints, successCheckPoints
}
