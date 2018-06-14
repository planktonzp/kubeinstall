package cluster

import (
	"kubeinstall/src/logdebug"
	"kubeinstall/src/runconf"
	"kubeinstall/src/session"
	"sync"
)

//7步安装k8s集群
const (
	Step1     = "step1CheckInstallPlan"
	Step2     = "step2CreateYUMStorage"     //支持自定义(无法跳过)
	Step3     = "step3InstallRPMs"          //不支持自定义(包括集群“每个节点”都需要的各个软件rpm包)
	Step4     = "step4CreateDockerRegistry" //支持自定义()
	Step5     = "step5InstallEtcd"          //etcd安装
	Step6     = "step6MasterInit"           //暂时不支持自定义(无法跳过)
	Step7     = "step7NodesJoin"            //暂时不支持列表外的node
	Step8     = "step8InstallCeph"          //一键安装ceph 应该在k8s集群稳定运行后安装
	Step9     = "step9InstallK8sModules"    //安装k8s相关组件
	StepEX5   = "stepEX5InstallEtcd"        //扩展etcd安装
	StepEX7   = "stepEX7NodesJoin"          //扩展Node加入集群
	StepEX8   = "stepEX8InstallCeph"        //扩展安装ceph
	StepXKill = "stepXKill"
	StepXTest = "stepXTest"
)

type moduleInfo struct {
	SessionIDMap map[string]int `json:"sessionIDMap"`
	mutex        *sync.Mutex
}

var modules moduleInfo

func initMoulesInfo() {
	modules.mutex = new(sync.Mutex)

	modules.SessionIDMap = map[string]int{
		Step1:   session.InvaildSessionID,
		Step2:   session.InvaildSessionID,
		Step3:   session.InvaildSessionID,
		Step4:   session.InvaildSessionID,
		Step5:   session.InvaildSessionID,
		Step6:   session.InvaildSessionID,
		Step7:   session.InvaildSessionID,
		Step8:   session.InvaildSessionID,
		Step9:   session.InvaildSessionID,
		StepEX5: session.InvaildSessionID,
		StepEX7: session.InvaildSessionID,
		StepEX8: session.InvaildSessionID,
	}

	//尝试恢复数据
	recoverData := make(map[string]int, 0)
	err := runconf.Read(runconf.DataTypeModuleSessionIDMap, &recoverData)
	if err == nil {
		logdebug.Println(logdebug.LevelInfo, "尝试恢复sessionMap数据")

		modules.SessionIDMap = recoverData
	}

	return
}

//GetModuleSessionID 获取模块的sessionID
func GetModuleSessionID(step string) int {
	modules.mutex.Lock()

	defer modules.mutex.Unlock()

	sessionID, ok := modules.SessionIDMap[step]
	if !ok {
		return session.InvaildSessionID
	}

	return sessionID
}

//UpdateModules 更新模块session表项
func UpdateModules(sessionID int, step string) {
	modules.mutex.Lock()

	defer modules.mutex.Unlock()

	modules.SessionIDMap[step] = sessionID
	runconf.Write(runconf.DataTypeModuleSessionIDMap, modules.SessionIDMap)

	return
}

//DeleteModuleSessionID 删除一个step对应的session表项
func DeleteModuleSessionID(sessionID int) {
	modules.mutex.Lock()

	defer modules.mutex.Unlock()

	for step, v := range modules.SessionIDMap {
		if v == sessionID {
			//delete(modules.SessionIDMap, step)
			modules.SessionIDMap[step] = session.InvaildSessionID
			runconf.Write(runconf.DataTypeModuleSessionIDMap, modules.SessionIDMap)

			return
		}
	}

	return
}

//DeleteAllModuleSessionID 删除所有模块的session表项
func DeleteAllModuleSessionID() {
	modules.mutex.Lock()

	defer modules.mutex.Unlock()

	for step := range modules.SessionIDMap {
		modules.SessionIDMap[step] = session.InvaildSessionID
	}

	runconf.Write(runconf.DataTypeModuleSessionIDMap, modules.SessionIDMap)

	return
}
