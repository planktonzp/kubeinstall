package apiserver

import (
	"kubeinstall/src/apiserver/createcluster"
	"kubeinstall/src/apiserver/modifycluster"
	"kubeinstall/src/apiserver/nodequery"
	"kubeinstall/src/apiserver/removecluster"
	"kubeinstall/src/apiserver/taskquery"
	"kubeinstall/src/config"
	"log"
	"net/http"
)

//Server APIServer
type Server struct {
	HTTPServerPort string
	creator        createcluster.Server
	modifier       modifycluster.Server
	remover        removecluster.Server
	taskQuerier    taskquery.Server
	nodeQuerier    nodequery.Server
}

//ListenAndServe 开启HTTP监听服务
func (svc *Server) ListenAndServe() {
	serverAddr := ":" + svc.HTTPServerPort

	log.Fatal(http.ListenAndServe(serverAddr, nil))

	return
}

//Init 初始化APIServer
func (svc *Server) Init() {
	svc.HTTPServerPort = config.GetAPIServerPort()

	svc.creator.Register()

	svc.modifier.Register()

	svc.remover.Register()

	svc.taskQuerier.Register()

	svc.nodeQuerier.Register()

	return
}
