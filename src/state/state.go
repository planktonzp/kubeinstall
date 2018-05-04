package state

import (
	"kubeinstall/src/logdebug"
	"kubeinstall/src/runconf"
	"kubeinstall/src/session"
	"sync"
)

//任务当前状态
const (
	Running  = "running"  //正在执行任务
	Error    = "error"    //执行出错退出
	Complete = "complete" //完成
)

//Context 任务执行上下文(包括stdout stderr errMsg state等 后续可拓展)
type Context struct {
	Stdout          string `json:"stdout"` //标准输出
	Stderr          string `json:"stderr"` //标准错误
	ErrMsg          string `json:"errMsg"` //内部错误信息(代码级别)
	State           string `json:"state"`  //任务当前状态
	Tips            string `json:"tips"`   //提示信息
	SchedulePercent string `json:"schedulePercent"`
}

//tasks 任务信息
type tasks struct {
	ContextSet map[int]Context `json:"contextSet"`
	mutex      *sync.Mutex
}

//全局信息
var globalTasks tasks

//Init 初始化全局任务信息表
func Init() {
	globalTasks.mutex = new(sync.Mutex)
	globalTasks.ContextSet = make(map[int]Context, 0)

	recoverData := make(map[int]Context, 0)
	err := runconf.Read(runconf.DataTypeTaskContext, &recoverData)
	if err == nil {
		logdebug.Println(logdebug.LevelInfo, "尝试恢复context数据")
		globalTasks.ContextSet = recoverData
	}

	return
}

//Get 获取任务上下文
func Get(sessionID int) Context {
	globalTasks.mutex.Lock()

	defer globalTasks.mutex.Unlock()

	context := globalTasks.ContextSet[sessionID]

	return context
}

//Update 更新任务上下文(追加)
func Update(sessionID int, newContext Context) {
	globalTasks.mutex.Lock()

	defer globalTasks.mutex.Unlock()

	if sessionID == session.InvaildSessionID {
		return
	}

	oldContext := globalTasks.ContextSet[sessionID]

	//追加链接新的输出 更新状态位（由step控制状态）
	oldContext.Stderr += newContext.Stderr

	oldContext.Stdout += newContext.Stdout

	oldContext.ErrMsg += newContext.ErrMsg
	oldContext.Tips += newContext.Tips

	oldContext.State = newContext.State
	//某些回退场景没有填写进度
	if newContext.SchedulePercent != "" {
		oldContext.SchedulePercent = newContext.SchedulePercent
	}

	globalTasks.ContextSet[sessionID] = oldContext
	runconf.Write(runconf.DataTypeTaskContext, globalTasks.ContextSet)

	return
}

//Delete 销毁上下文
func Delete(sessionID int) {
	globalTasks.mutex.Lock()

	defer globalTasks.mutex.Unlock()

	delete(globalTasks.ContextSet, sessionID)
	runconf.Write(runconf.DataTypeTaskContext, globalTasks.ContextSet)

	return
}

//GetAll 获取所有上下文(直接返回map可能存在隐患)
func GetAll() map[int]Context {
	globalTasks.mutex.Lock()

	defer globalTasks.mutex.Unlock()

	return globalTasks.ContextSet
}

//DeleteAll 删除所有上下文
func DeleteAll() {
	globalTasks.mutex.Lock()

	defer globalTasks.mutex.Unlock()

	for sessionID := range globalTasks.ContextSet {
		delete(globalTasks.ContextSet, sessionID)
	}
	runconf.Write(runconf.DataTypeTaskContext, globalTasks.ContextSet)

	return
}
