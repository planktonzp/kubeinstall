package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	//"strings"
	"strings"
)

type info struct {
	Name string
}

func main() {
	var dataContent info
	savePath := "./1.json"

	file, err := os.Open(savePath)
	if err != nil {
		return
	}

	defer file.Close()

	dataJSON, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}

	err = json.Unmarshal(dataJSON, &dataContent)

	fmt.Println("----old-name----", dataContent.Name)
	newStr := strings.Replace(dataContent.Name, "$(IP)", "192.168.0.0", -1)
	fmt.Println("----new-name----", newStr)

	return
}
