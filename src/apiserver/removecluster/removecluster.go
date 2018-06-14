package removecluster

import (
	"github.com/emicklei/go-restful"
	"kubeinstall/src/cluster"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/session"
	//"kubeinstall/src/msg"
	"net/http"
	//"strconv"
	//"golang.org/x/tools/go/gcimporter15/testdata"
	"kubeinstall/src/msg"
	"kubeinstall/src/state"
)

//7步安装k8s集群 - 反操作 执行顺序以及作用域必须与正向操作绑定

//参数中的各个字段与当前集群状态对比 如果是外部的资源 则只能初始化依赖它的节点配置 不能对该资源清理
type callback func(moduleSessionID int, plan cluster.InstallPlan)

//Server removeCluster API Server
type Server struct {
	callbackMap map[string]callback
}

func removeYUMStorage(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveYUMStorage(moduleSessionID)

	return
}

//最终会使用SSH登录清理k8scc ，参数中必需提供ssh登录信息
func removeRPMs(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveRPMs(moduleSessionID)

	return
}

func removeDockerRegistry(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveDockerRegistry(moduleSessionID)

	return
}

func removeEtcd(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveEtcd(moduleSessionID)

	return
}

func removeMaster(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveMaster(moduleSessionID)

	return
}

func removeNodes(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveNodes(moduleSessionID)

	return
}

func removeCeph(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.RemoveCeph(moduleSessionID)

	return
}

func (svc *Server) execRemoveStep(request *restful.Request, response *restful.Response) {
	var plan cluster.InstallPlan
	//var output string

	err := request.ReadEntity(&plan)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)

		return
	}

	resp := msg.Response{
		Result: true,
	}

	step := request.PathParameter("step")
	optFunc, ok := svc.callbackMap[step]
	if !ok {
		resp.Result = false
		resp.ErrorMsg = "非法的操作步骤"
		response.WriteEntity(resp)

		return
	}

	//没有申请过 或者已经存在 但为初始值
	//moduleSessionID := cluster.GetModuleSessionID(step)
	//if moduleSessionID == session.InvaildSessionID {
	moduleSessionID := session.Alloc()
	cluster.UpdateModules(moduleSessionID, step)
	//}

	context := state.Context{
		State:           state.Running,
		SchedulePercent: "0%",
	}
	//任务开始....
	state.Update(moduleSessionID, context)

	optFunc(moduleSessionID, plan)
	resp.Content = plan
	resp.SessionID = moduleSessionID

	return
}

func (svc *Server) webRegister() {
	ws := new(restful.WebService)

	ws.
		Path("/cluster/remove").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.POST("/{step}").To(svc.execRemoveStep).
		// docs
		Doc("exec a step to remove").
		Operation("removeCluster"))

	//解决js跨域问题 假设kubeinstall与BCM在同一台机器部署
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

//Register 注册web服务
func (svc *Server) Register() {
	logdebug.Println(logdebug.LevelInfo, "注册web服务!")

	svc.callbackMap = map[string]callback{
		cluster.Step2: removeYUMStorage,
		cluster.Step3: removeRPMs,
		cluster.Step4: removeDockerRegistry,
		cluster.Step5: removeEtcd,
		cluster.Step6: removeMaster,
		cluster.Step7: removeNodes,
		cluster.Step8: removeCeph,
	}

	svc.webRegister()

	return
}
