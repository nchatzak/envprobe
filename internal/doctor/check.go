package doctor

import (
	"os/exec"
)

type Result struct {
	Name  string
	Found bool
	Path  string
}

func CheckAll(tools []string) []Result {
	results := make([]Result, 0, len(tools))
	for _, tool := range tools {
		results = append(results, checkTool(tool))
	}

	return results
}

func checkTool(tool string) Result {
	path, err := exec.LookPath(tool)
	return Result{
		Name:  tool,
		Found: err == nil,
		Path:  path,
	}
}
