package domain

type Discovery struct {
	OS           string   `json:"os"`
	Hostname     string   `json:"hostname"`
	CPU          string   `json:"cpu"`
	Memory       string   `json:"memory"`
	Disk         string   `json:"disk"`
	Containers   []string `json:"containers"`
	Capabilities []string `json:"capabilities"`
}

type ToolRequest struct {
	Tool      string            `json:"tool"`
	Arguments map[string]string `json:"arguments"`
}

type ToolResult struct {
	Tool    string `json:"tool"`
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
