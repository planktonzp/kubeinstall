package cmd

import (
	//"io/ioutil"
	"fmt"
	//"github.com/docker/docker/integration-cli/checker"
	"github.com/pkg/errors"
	"io/ioutil"
	"kubeinstall/src/kubeworker/logdebug"
	"os/exec"
	"strings"
)

//ExecInfo 命令执行信息
type ExecInfo struct {
	CMDContent  string   `json:"cmdContent"`
	ErrorTags   []string `json:"errorTags"`
	SuccessTags []string `json:"successTags"`
	CMDHostAddr string   `json:"cmdHostAddr"`
	EnvSet      []string `json:"envSet"`
}

//根据错误列表 检查内容中是否包含错误;
//如果没有检查出错 则检查成功判定元素;
func (e *ExecInfo) checkOutputContent(output string) bool {
	for _, errorKey := range e.ErrorTags {
		if strings.Contains(output, errorKey) {
			//包含错误 返回失败
			return false
		}
	}

	for _, successKey := range e.SuccessTags {
		if !strings.Contains(output, successKey) {
			//缺少正确判定key
			return false
		}
	}

	return true
}

//Exec 执行linux命令
func (e *ExecInfo) Exec() (string, error) {

	logdebug.Println(logdebug.LevelDebug, "---test---cmd---", e.CMDContent)

	c := exec.Command("bash", []string{"-c", e.CMDContent}...)

	stdoutReader, _ := c.StdoutPipe()
	stderrReader, _ := c.StderrPipe()

	for _, env := range e.EnvSet {
		c.Env = append(c.Env, env)
	}

	err := c.Start()
	if err != nil {
		return err.Error(), err
	}

	stdOutput, _ := ioutil.ReadAll(stdoutReader)
	if err != nil {
		return err.Error(), err
	}

	errOutput, err := ioutil.ReadAll(stderrReader)
	if err != nil {
		return err.Error(), err
	}

	c.Wait()
	output := string(stdOutput) + string(errOutput)

	logdebug.Println(logdebug.LevelInfo, "----命令执行结果err=", string(output))

	result := e.checkOutputContent(output)
	if result != true {
		errMsg := fmt.Sprintf(`Exec CMD [%s] failed!`, e.CMDContent)

		return string(output), errors.New(errMsg)
	}

	return string(output), nil
}
