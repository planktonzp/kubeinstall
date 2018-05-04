package main

import (
	"kubeinstall/src/kubeworker/apiserver"
	"kubeinstall/src/kubeworker/config"
	"kubeinstall/src/kubeworker/logdebug"
	//"kubeinstall/src/kubeworker/network"
)

type kubeWorker struct {
	svc apiserver.Server
}

var worker kubeWorker

func init() {
	config.Init()

	worker.svc.Init()

	//network.Init()
}

func main() {
	config.PrintStartConfig()

	logdebug.Println(logdebug.LevelInfo, "-----kube worker start!----")

	worker.svc.ListenAndServe()

	return
}
