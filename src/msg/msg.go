package msg

import (
	"encoding/json"
	//"io"
	"io/ioutil"
	//"log"
	"net/http"
	"strings"
)

//Response 回复结构
type Response struct {
	Result    bool        `json:"result"`
	ErrorMsg  string      `json:"errorMsg"`
	Output    string      `json:"output"`
	Content   interface{} `json:"content"`
	SessionID int         `json:"sessionID"`
}

//PUT restful请求PUT操作
const (
	PUT    = "PUT"
	POST   = "POST"
	GET    = "GET"
	DELETE = "DELETE"
)

const (
	applicationTypeJSON = "application/json"
	applicationTypeXML  = "application/xml"
)

const httpHeaderContentType string = "Content-Type"

const httpHeaderAccept string = "Accept"

//Request 请求结构
type Request struct {
	URL     string      `json:"url"`
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

//SendRequestByJSON 用于发送json格式的http请求
func (reqInfo *Request) SendRequestByJSON() ([]byte, error) {
	//log.Println("content =", content)
	jsonTypeContent, _ := json.Marshal(reqInfo.Content)
	body := strings.NewReader(string(jsonTypeContent))

	client := &http.Client{}

	//log.Println("jsonTypeContent = ", string(jsonTypeContent))

	req, _ := http.NewRequest(reqInfo.Type, reqInfo.URL, body)
	req.Header.Set(httpHeaderContentType, applicationTypeJSON)
	req.Header.Set(httpHeaderAccept, applicationTypeJSON)

	//log.Printf("[看下发送的结构]%+v\n", req) //看下发送的结构

	resp, err := client.Do(req) //发送
	if err != nil {
		return []byte{}, err
	}
	//resp, _ := client.Do(req) //发送
	defer resp.Body.Close() //一定要关闭resp.Body
	data, _ := ioutil.ReadAll(resp.Body)

	respBody := data
	//
	//log.Println("[put之后 收到的数据]", string(data), err)

	//log.Println("respBody=-------", respBody)

	return respBody, err
}
