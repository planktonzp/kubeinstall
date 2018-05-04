package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

func main() {
	os.Setenv("TEST_ENV", "hyptoknwm")

	fmt.Println("TEST_ENV========", os.Getenv("TEST_ENV"))

	cmd := exec.Command("bash", []string{"-c", "/home/hyp/work/src/kubeinstall/files/sh/1.sh"}...)

	reader, _ := cmd.StdoutPipe()

	cmd.Start()

	cmdOutput, _ := ioutil.ReadAll(reader)

	cmd.Wait()

	fmt.Println("---------cmdOutput-------", string(cmdOutput))

	os.Unsetenv("TEST_ENV")

	fmt.Println("TEST_ENV========", os.Getenv("TEST_ENV"))
}
