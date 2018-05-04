package execute

import (
	"github.com/emicklei/go-restful"
	"kubeinstall/src/kubeworker/cmd"
	//"kubeinstall/src/kubeworker/config"
	"kubeinstall/src/kubeworker/logdebug"
	"net/http"
)

//Server 准备工作服务
type Server struct {
}

//Response 回复结构
type Response struct {
	Result   bool        `json:"result"`
	ErrorMsg string      `json:"errorMsg"`
	Output   string      `json:"output"`
	Content  interface{} `json:"content"`
}

func (svc *Server) execShellCMD(request *restful.Request, response *restful.Response) {
	var execInfo cmd.ExecInfo
	var output string

	err := request.ReadEntity(&execInfo)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)

		return
	}

	logdebug.Println(logdebug.LevelInfo, "----收到请求---", execInfo)

	resp := Response{
		Result: true,
	}
	output, err = execInfo.Exec()
	if err != nil {
		resp.Result = false
		resp.ErrorMsg = err.Error()
	}

	logdebug.Println(logdebug.LevelInfo, output, err)

	resp.Output = output
	resp.Content = execInfo

	response.WriteEntity(resp)

	return
}

//Register 注册服务
func (svc *Server) Register() {
	ws := new(restful.WebService)

	ws.
		Path("/exec").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.POST("/").To(svc.execShellCMD).
		// docs
		Doc("执行shell命令").
		Operation("execShellCMD").
		Reads(cmd.ExecInfo{})) // from the request

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
