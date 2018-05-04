package session

import (
	//"kubeinstall/src/config"
	//"kubeinstall/src/logdebug"
	"sync"
)

type sessionInfo struct {
	sessionIDSet map[int]bool
	mutex        *sync.Mutex
}

//InvaildSessionID 非法的sessionID
const InvaildSessionID int = -1

var globalSession sessionInfo

//Init 初始化全局session表
func Init() {
	globalSession.mutex = new(sync.Mutex)
	globalSession.sessionIDSet = make(map[int]bool, 0)

	return
}

//Alloc 申请sessionID
func Alloc() int {
	sessionID := 0

	globalSession.mutex.Lock()

	defer globalSession.mutex.Unlock()

	for {

		ok := globalSession.sessionIDSet[sessionID]
		if !ok {
			//没有保存过这个sessionID 则新存入
			globalSession.sessionIDSet[sessionID] = true

			return sessionID
		}

		sessionID++
	}
}

//Free 释放sessionID
func Free(sessionID int) {
	globalSession.mutex.Lock()

	defer globalSession.mutex.Unlock()

	delete(globalSession.sessionIDSet, sessionID)

	return
}

//Get 获取全局session表 用于测试
func Get() map[int]bool {
	return globalSession.sessionIDSet
}
