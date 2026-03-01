package genui

import "github.com/overhuman/overhuman/internal/pipeline"

// makeAPIRunResult creates a pipeline.RunResult from an API generate request.
func makeAPIRunResult(req apiGenerateRequest) pipeline.RunResult {
	return pipeline.RunResult{
		TaskID:       req.TaskID,
		Success:      true,
		Result:       req.Result,
		QualityScore: req.QualityScore,
	}
}
