package servicecatalog

import "time"

// PipelineRun is the minimal projection servicecatalog
// needs from a pipeline.PipelineRun. We keep it here (not
// imported from the pipeline package) so this package
// stays free of cross-package coupling. Production code
// (cmd/devops-toolkit/main.go) maps the real
// pipeline.PipelineRun row onto this shape.
type PipelineRun struct {
	ID         string
	Status     string
	StartedAt  time.Time
	DurationMs int64
}

// PipelineRunsForService is the function-typed seam main.go
// satisfies when wiring the health endpoint. The
// service-catalog package cannot import the pipeline
// package (cycle — pipeline has no dependency on us), so
// the production wiring is done by a closure that calls
// pipeline.Repository.LastRunsForService and converts the
// rows.
type PipelineRunsForService func(serviceID string, n int) ([]PipelineRun, error)

// FunRunSource is the RunSource implementation backed by
// a PipelineRunsForService function. The "Fun" name is a
// nod to its role: a pure-function adapter, not a real
// data source.
type FunRunSource struct {
	Fn PipelineRunsForService
}

// LastNRunsForService satisfies RunSource.
func (f *FunRunSource) LastNRunsForService(serviceID string, n int) ([]runRow, error) {
	rows, err := f.Fn(serviceID, n)
	if err != nil {
		return nil, err
	}
	out := make([]runRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, runRow{
			ID:         r.ID,
			Status:     r.Status,
			StartedAt:  r.StartedAt,
			DurationMs: r.DurationMs,
		})
	}
	return out, nil
}
