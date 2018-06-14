package msg

import (
	"io/ioutil"
	"fmt"
)

func test() {
	POSTData := struct {
		Account string `json:"account"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}{
		Account: "张锋1",
		Title:   "测试  alert_rtx",
		Content: "i am rtx",
	}

	jsonTypeContent, _ := json.Marshal(POSTData)
	body := strings.NewReader(string(jsonTypeContent))
	client := &http.Client{}
	req, _ := http.NewRequest("POST", "http://api.msg.ifengidc.com:7989/sendRTX", body)
	req.Header.Add("authToken", authToken)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(data, &respMsg)
	fmt.Println("respData from rtx", respMsg)
}