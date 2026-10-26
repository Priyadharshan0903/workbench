package catalog

// RegistrationResult is the CLI/MCP acknowledgement after a successful save.
type RegistrationResult struct {
	OK       bool   `json:"ok"`
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
