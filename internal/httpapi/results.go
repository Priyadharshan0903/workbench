package httpapi

type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type engineHealth struct {
	OK          bool   `json:"ok"`
	App         string `json:"app"`
	Version     int    `json:"version"`
	Runtime     string `json:"runtime"`
	SourceStamp string `json:"sourceStamp"`
	SourceRoot  string `json:"sourceRoot"`
}

type processIdentity struct {
	PID       int      `json:"pid"`
	Name      string   `json:"name"`
	Birth     string   `json:"birth"`
	CWD       string   `json:"cwd"`
	Endpoints []string `json:"endpoints"`
	ProjectID *string  `json:"projectId"`
	UID       *int     `json:"uid,omitempty"`
}
