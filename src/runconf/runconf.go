package runconf

import (
	"encoding/json"
	"kubeinstall/src/config"
	//"kubeinstall/src/logdebug"

	"io/ioutil"
	"os"
)

//DataTypeYumCMDConf yum模块相关shell命令配置文件
const (
	DataTypeYumCMDConf           = "yumcmd.json"
	DataTypeRpmsCMDConf          = "rpmscmd.json"
	DataTypeKubeWorkerCMDConf    = "kubeworker-cmd.json"
	DataTypeCurrentClusterStatus = "current-cluster-status.json"
	DataTypeModuleSessionIDMap   = "module-sessionid-map.json"
	DataTypeTaskContext          = "task-context.json"
	DataTypeDNSMonitorCMDConf    = "dns-monitor-cmd.json"
	DataTypeHeapsterCMDConf      = "heapster-cmd.json"
	DataTypeInfluxdbCMDConf      = "influxdb-cmd.json"
	DataTypePrometheusCMDConf    = "promutheus-cmd.json"
	DataTypeRedisCMDConf         = "redis-cmd.json"
)

//Write 写入运行配置
func Write(dataType string, dataContent interface{}) (err error) {
	savePath := config.GetRunConfDirPath() + dataType

	os.Remove(savePath)

	os.MkdirAll(config.GetRunConfDirPath(), os.ModePerm)

	//jsonTypeContent, _ := json.Marshal(dataContent)
	//排版后输出到文件里
	jsonTypeContent, _ := json.MarshalIndent(dataContent, "", "  ")

	//logdebug.Println(logdebug.LevelInfo, "-------备份数据------", dataContent)

	//err = ioutil.WriteFile(savePath, jsonTypeContent, os.ModeAppend) //写入文件(字节数组)
	err = ioutil.WriteFile(savePath, jsonTypeContent, os.ModePerm) //写入文件(字节数组)

	//logdebug.Println(logdebug.LevelError, err)

	return
}

//Read 读取运行配置
func Read(dataType string, dataContent interface{}) error {
	savePath := config.GetRunConfDirPath() + dataType

	file, err := os.Open(savePath)
	if err != nil {
		return err
	}

	defer file.Close()

	dataJSON, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(dataJSON, dataContent)

	return err
}
