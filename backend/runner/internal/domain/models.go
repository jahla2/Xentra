package domain

type Discovery struct { OS string `json:"os"`; Hostname string `json:"hostname"`; Capabilities []string `json:"capabilities"` }
type ToolRequest struct { Tool string `json:"tool"`; Arguments map[string]string `json:"arguments"` }
type ToolResult struct { Tool string `json:"tool"`; Success bool `json:"success"`; Output string `json:"output"`; Error string `json:"error,omitempty"` }
