package param

type MoveFileParam struct {
	Source string `json:"source" binding:"required"` // 源文件路径
	Target string `json:"target" binding:"required"` // 目标文件路径
}
