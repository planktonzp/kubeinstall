package apiserver

import (
	"kubeinstall/src/kubeworker/apiserver/execute"
	"kubeinstall/src/kubeworker/config"
	"kubeinstall/src/kubeworker/logdebug"
	"log"
	"net/http"
)

//Server 各种APIServer
type Server struct {
	HTTPServerPort string
	execSvc        execute.Server
}

//Init Http服务注册总入口
func (svc *Server) Init() {
	//由kubeinstall在安装启动时指定了端口
	svc.HTTPServerPort = config.GetAPIServerPort()

	logdebug.Println(logdebug.LevelInfo, "APIServer服务初始化!")

	svc.execSvc.Register()

	return
}

//ListenAndServe 监听服务开启
func (svc *Server) ListenAndServe() {
	serverAddr := ":" + svc.HTTPServerPort

	log.Fatal(http.ListenAndServe(serverAddr, nil))

	return
}
