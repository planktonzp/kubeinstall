package network

import (
	"kubeinstall/src/kubeworker/cmd"
	"kubeinstall/src/kubeworker/config"
	"kubeinstall/src/kubeworker/logdebug"
)

func Init() {

	errorTips, successTips := config.GetStopServiceCheckPoints()

	fwStopCMD := config.GetFireWallStopCMD()

	output, err := cmd.Exec(fwStopCMD, errorTips, successTips)

	logdebug.Println(logdebug.LevelInfo, output, err)
}
