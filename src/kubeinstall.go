package main

import (
	//"fmt"
	"kubeinstall/src/apiserver"
	"kubeinstall/src/config"
)

//KubeInstaller 一键安装k8s集群工具结构
type KubeInstaller struct {
	svc apiserver.Server
}

var installer KubeInstaller

var _Version_ = "UNKNOWN"
var _BuildTime_ = "UNKNOWN"

func init() {

	config.Init(_Version_, _BuildTime_)

	installer.svc.Init()

}

func main() {
	//fmt.Println("Hello KubeInstall!")

	//缺少参数合法性检查

	config.PrintStartConfig()

	installer.svc.ListenAndServe()

	return
}
