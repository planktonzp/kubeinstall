package modifycluster

import (
	//"github.com/emicklei/go-restful"
	//"github.com/emicklei/go-restful"
	"kubeinstall/src/cluster"
	"kubeinstall/src/logdebug"
)

type callback func(plan cluster.InstallPlan) (string, error)

//Server modifyCluster API Server
type Server struct {
	callbackMap map[string]callback
}

//Register 注册web服务
func (svc *Server) Register() {
	logdebug.Println(logdebug.LevelInfo, "注册web服务!")
}

func (svc *Server) webRegister() {
	return
}
