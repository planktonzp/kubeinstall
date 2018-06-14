package nodequery

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

	//"kubeinstall/src/cluster"
	//"kubeinstall/src/session"
	"kubeinstall/src/state"
	"net/http"
	"sync"
	//"strconv"
)

type nodeContext struct {
	Context state.Context `json:"context"`
	Step    string        `json:"step"`
	HostIP  string        `json:"hostIP"`
}

//Server node查询服务数据
type Server struct {
	AllNodesContextSet map[string]map[string]nodeContext
	CurrentStep        string
	mutex              *sync.Mutex
}

//Register 服务注册
func (svc *Server) Register() {
	svc.AllNodesContextSet = make(map[string]map[string]nodeContext, 0)
	svc.mutex = new(sync.Mutex)

	svc.webRegister()

	return
}

func (svc *Server) getAllNodesState(request *restful.Request, response *restful.Response) {
	svc.mutex.Lock()
	defer svc.mutex.Unlock()

	response.WriteHeaderAndJson(200, svc.AllNodesContextSet, "application/json")

	return
}

func (svc *Server) getNodeState(request *restful.Request, response *restful.Response) {
	svc.mutex.Lock()
	defer svc.mutex.Unlock()

	nodeIP := request.PathParameter("ip")

	response.WriteHeaderAndJson(200, svc.AllNodesContextSet[nodeIP], "application/json")

	return
}

func (svc *Server) getNodeStateStep(request *restful.Request, response *restful.Response) {
	svc.mutex.Lock()
	defer svc.mutex.Unlock()

	nodeIP := request.PathParameter("ip")

	current := svc.AllNodesContextSet[nodeIP]

	step := request.PathParameter("step")

	response.WriteHeaderAndJson(200, current[step], "application/json")

	return
}

func (svc *Server) addNodeState(request *restful.Request, response *restful.Response) {
	nodeState := nodeContext{}
	svc.mutex.Lock()
	defer svc.mutex.Unlock()

	err := request.ReadEntity(&nodeState)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)

		return
	}

	current := svc.AllNodesContextSet[nodeState.HostIP]
	if len(current) == 0 {
		current = make(map[string]nodeContext, 0)
	}

	current[nodeState.Step] = nodeState

	svc.AllNodesContextSet[nodeState.HostIP] = current
	svc.CurrentStep = nodeState.Step

	return
}

func (svc *Server) updateNodeState(request *restful.Request, response *restful.Response) {
	nodeIP := request.PathParameter("ip")
	nodeState := nodeContext{}
	svc.mutex.Lock()
	defer svc.mutex.Unlock()

	err := request.ReadEntity(&nodeState)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)

		return
	}

	current, ok := svc.AllNodesContextSet[nodeIP]
	if !ok {
		response.WriteEntity("找不到指定的IP的状态!")

		return
	}
	//用户更新时没有配置step 则使用最近一次POST的step
	step := nodeState.Step
	if step == "" {
		step = svc.CurrentStep
	}

	stepState := current[step]

	stepState.Context.Stdout += nodeState.Context.Stdout
	stepState.Context.Stderr += nodeState.Context.Stderr
	stepState.Context.ErrMsg += nodeState.Context.ErrMsg
	stepState.Context.Tips += nodeState.Context.Tips

	if nodeState.Context.State != "" {
		stepState.Context.State = nodeState.Context.State
	}

	if nodeState.Context.SchedulePercent != "" {
		stepState.Context.SchedulePercent = nodeState.Context.SchedulePercent
	}

	current[step] = stepState

	svc.AllNodesContextSet[nodeIP] = current

	return
}

func (svc *Server) deleteNodeStateStep(request *restful.Request, response *restful.Response) {
	nodeIP := request.PathParameter("ip")
	current, ok := svc.AllNodesContextSet[nodeIP]
	if !ok {
		response.WriteEntity("找不到指定的IP的状态!")

		return
	}

	step := request.PathParameter("step")

	delete(current, step)

	svc.AllNodesContextSet[nodeIP] = current

	if len(current) == 0 {
		delete(svc.AllNodesContextSet, nodeIP)
	}

	return
}

func (svc *Server) webRegister() {
	ws := new(restful.WebService)

	ws.
		Path("/nodequery").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.GET("/").To(svc.getAllNodesState).
		// docs
		Doc("get all node state").
		Operation("getAllNodesState").
		Returns(200, "OK", map[string]map[string]nodeContext{}))

	ws.Route(ws.GET("/{ip}").To(svc.getNodeState).
		// docs
		Doc("get single node state").
		Operation("getTaskState").
		Returns(200, "OK", map[string]nodeContext{}))

	ws.Route(ws.GET("/{ip}/{step}").To(svc.getNodeStateStep).
		// docs
		Doc("get single node state").
		Operation("getNodeStateStep").
		Returns(200, "OK", nodeContext{}))

	ws.Route(ws.POST("/").To(svc.addNodeState).
		// docs
		Doc("add a node state").
		Operation("addNodeState").
		Reads(nodeContext{})) // from the request

	ws.Route(ws.PUT("/{ip}").To(svc.updateNodeState).
		// docs
		Doc("update node state").
		Operation("updateNodeState").
		Param(ws.PathParameter("ip", "identifier of the app server").DataType("string")).
		Reads(nodeContext{})) // from the request

	//ws.Route(ws.DELETE("/").To(svc.deleteAllNodesState).
	//	// docs
	//	Doc("delete all nodes state").
	//	Operation("deleteAllNodesState "))
	//
	//ws.Route(ws.DELETE("/{ip}").To(svc.deleteNodeState).
	//	// docs
	//	Doc("delete a node state").
	//	Operation("deleteNodeState"))

	ws.Route(ws.DELETE("/{ip}/{step}").To(svc.deleteNodeStateStep).
		// docs
		Doc("delete a node state").
		Operation("deleteNodeState"))

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
