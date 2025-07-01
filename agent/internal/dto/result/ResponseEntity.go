package result

type Resp struct {
	Code    int    `json:"code"`    // 响应码
	Data    string `json:"data"`    // 响应数据
	Message string `json:"message"` // 响应消息

}

func NewSuccessRespD(data string) *Resp {
	return &Resp{Code: 200, Data: data, Message: "操作成功"}
}
func NewSuccessResp() *Resp {
	return &Resp{Code: 200, Data: "", Message: "操作成功"}
}
func NewFailRespM(message string) *Resp {
	return &Resp{Code: 500, Data: "", Message: message}
}
func NewFailResp() *Resp {
	return &Resp{Code: 500, Data: "", Message: "操作失败"}
}
