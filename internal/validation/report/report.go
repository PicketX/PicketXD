package report

import "io"

type ScenarioResult struct {
	Name, Status, Detail         string
	Selected, Deleted, Remaining int
}
type Report struct {
	Kernel, Go string
	Libraries  map[string]string
	Scenarios  []ScenarioResult
}

func Write(io.Writer, Report) error { return nil }
