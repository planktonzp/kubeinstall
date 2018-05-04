package createcluster

import (
	//"fmt"
	//"encoding/json"
	//"fmt"
	"github.com/emicklei/go-restful"
	//"github.com/pkg/errors"
	"kubeinstall/src/cluster"
	//"kubeinstall/src/config"
	"kubeinstall/src/logdebug"
	"kubeinstall/src/msg"

	//"kubeinstall/src/session"
	//"kubeinstall/src/runconf"
	"kubeinstall/src/config"
	"kubeinstall/src/session"
	"kubeinstall/src/state"
	"net/http"
)

type callback func(moduleSessionID int, plan cluster.InstallPlan)

//Server createCluster API Server
type Server struct {
	callbackMap map[string]callback
}

//#Step 1 检查安装计划
func checkInstallPlan(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.CheckInstallPlan(moduleSessionID)

	return
}

//#Step 2 创建YUM源
func createYUMStorage(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.CreateYUMStorage(moduleSessionID)

	return
}

//#Step 3 安装必要的RPM包
func installRPMs(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallRPMS(moduleSessionID)

	return
}

//#Step 4 创建docker仓库
func createDockerRegistry(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.CreateDockerRegistry(moduleSessionID)

	return
}

//#Step 5 安装etcd(集群/非集群)
func installEtcd(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallEtcd(moduleSessionID)

	return
}

//#Step EX 5 拓展安装etcd
func installEtcdEX(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallEtcdEX(moduleSessionID)

	return
}

//#Step 6 安装master(HA/非HA)
func initMaster(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InitMaster(moduleSessionID)

	return
}

//#Step 7 node加入集群(当前集群)
func nodesJoin(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.NodesJoin(moduleSessionID)

	return
}

//#Step EX 7 拓展node加入集群(当前集群)
func nodesJoinEX(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.NodesJoinEX(moduleSessionID)

	return
}

//#Step 8 安装Ceph
func installCeph(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallCeph(moduleSessionID)

	return
}

//#Step 9 安装k8s组件
func installK8sModules(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallK8sModules(moduleSessionID)

	return
}

//#Step EX 8 拓展安装Ceph
func installCephEX(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.InstallCephEX(moduleSessionID)

	return
}

func killCurrentCMD(moduleSessionID int, plan cluster.InstallPlan) {
	go plan.Kill()

	return
}

//测试用例 API
func testAPI(moduleSessionID int, plan cluster.InstallPlan) {

	//testSftpDownload()
	//go testSessionID()

	testBuildCurrentStatus(plan.TestCfg.ClusterStatus)

	return
}

func (svc *Server) execStepOperation(request *restful.Request, response *restful.Response) {
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
		resp.ErrorMsg = "非法的操作步骤-无法找到处理函数"
		resp.SessionID = session.InvaildSessionID
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

	//所有波及的主机 清理指定的step
	for _, sshInfo := range plan.MachineSSHSet {
		req := msg.Request{
			URL:  "http://localhost:" + config.GetAPIServerPort() + "/nodequery/" + sshInfo.HostAddr + "/" + step,
			Type: msg.DELETE,
		}

		req.SendRequestByJSON()
	}

	optFunc(moduleSessionID, plan)
	resp.Content = plan
	resp.SessionID = moduleSessionID

	response.WriteEntity(resp)

	return
}

//Register 注册web服务
func (svc *Server) Register() {
	logdebug.Println(logdebug.LevelInfo, "注册web服务!")

	svc.callbackMap = map[string]callback{
		cluster.Step1:     checkInstallPlan,
		cluster.Step2:     createYUMStorage,
		cluster.Step3:     installRPMs,
		cluster.Step4:     createDockerRegistry,
		cluster.Step5:     installEtcd,
		cluster.Step6:     initMaster,
		cluster.Step7:     nodesJoin,
		cluster.Step8:     installCeph,
		cluster.Step9:     installK8sModules,
		cluster.StepEX5:   installEtcdEX,
		cluster.StepEX7:   nodesJoinEX,
		cluster.StepEX8:   installCephEX,
		cluster.StepXKill: killCurrentCMD,
		cluster.StepXTest: testAPI,
	}

	//svc.clusterStatus.Init()
	cluster.Init()

	svc.webRegister()

	return
}

func (svc *Server) getCurrentClusterStatus(request *restful.Request, response *restful.Response) {
	currentCluster := cluster.GetStatus()

	response.WriteHeaderAndJson(200, currentCluster, "application/json")

	return
}

func (svc *Server) webRegister() {
	ws := new(restful.WebService)

	ws.
		Path("/cluster/create").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.POST("/{step}").To(svc.execStepOperation).
		// docs
		Doc("检查界面提供的安装计划").
		Operation("checkInstallPlan").
		Param(ws.PathParameter("step", "操作步骤").DataType("string")).
		Reads(cluster.InstallPlan{})) // from the request

	ws.Route(ws.GET("/").To(svc.getCurrentClusterStatus).
		// docs
		Doc("get all apps all cfgs").
		Operation("findAllK8sAppsNginxCfgs").
		Returns(200, "OK", cluster.Status{}))

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

	return
}
