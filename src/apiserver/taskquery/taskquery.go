package taskquery

import (
	//"fmt"
	//"encoding/json"
	//"fmt"
	"github.com/emicklei/go-restful"
	//"github.com/pkg/errors"
	//"kubeinstall/src/cluster"
	//"kubeinstall/src/config"
	//"kubeinstall/src/logdebug"
	//"kubeinstall/src/msg"

	"kubeinstall/src/session"
	"kubeinstall/src/state"
	//"net/http"
	"kubeinstall/src/cluster"
	"strconv"
)

//Server 任务查询服务
type Server struct {
}

//Register 注册服务
func (svc *Server) Register() {
	session.Init()

	state.Init()

	svc.webRegister()

	return
}

func (svc *Server) getAllTasksState(request *restful.Request, response *restful.Response) {
	allTasksState := state.GetAll()

	response.WriteHeaderAndJson(200, allTasksState, "application/json")

	return
}

func (svc *Server) getTaskState(request *restful.Request, response *restful.Response) {
	param := request.PathParameter("sessionID")
	sessionID, _ := strconv.Atoi(param)

	taskState := state.Get(sessionID)

	response.WriteHeaderAndJson(200, taskState, "application/json")

	return
}

func (svc *Server) deleteAllTaskState(request *restful.Request, response *restful.Response) {
	state.DeleteAll()

	cluster.DeleteAllModuleSessionID()

	return
}

func (svc *Server) deleteTaskState(request *restful.Request, response *restful.Response) {
	param := request.PathParameter("sessionID")
	sessionID, _ := strconv.Atoi(param)

	state.Delete(sessionID)

	cluster.DeleteModuleSessionID(sessionID)

	return
}

func (svc *Server) webRegister() {
	ws := new(restful.WebService)

	ws.
		Path("/taskquery").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.GET("/").To(svc.getAllTasksState).
		// docs
		Doc("get all tasks state").
		Operation("getAllTasksState").
		Returns(200, "OK", map[int]state.Context{}))

	ws.Route(ws.GET("/{sessionID}").To(svc.getTaskState).
		// docs
		Doc("get single task state").
		Operation("getTaskState").
		Returns(200, "OK", state.Context{}))

	ws.Route(ws.DELETE("/").To(svc.deleteAllTaskState).
		// docs
		Doc("delete all tasks state").
		Operation("deleteAllTaskState "))

	ws.Route(ws.DELETE("/{sessionID}").To(svc.deleteTaskState).
		// docs
		Doc("delete a task state").
		Operation("deleteTaskState"))

	//解决js跨域问题 假设kubeinstall与BCM在同一台机器部署就不用考虑跨域问题了
	corsRule := restful.CrossOriginResourceSharing{
		ExposeHeaders:  []string{"X-My-Header"},
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		//AllowedDomains: []string{},
		CookiesAllowed: false,
		Container:      restful.DefaultContainer,
	}

	ws.Filter(corsRule.Filter)

	ws.Filter(restful.DefaultContainer.OPTIONSFilter)

	restful.Add(ws)
}
