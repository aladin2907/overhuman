package pipeline

import (
	"context"
	"fmt"
	"sync"
)

// DAGExecutor runs subtasks respecting dependency ordering.
// Independent subtasks execute in parallel via goroutines.
type DAGExecutor struct {
	// execFn is called for each subtask to produce its result.
	execFn func(ctx context.Context, subtask *SubtaskSpec) (string, error)
}

// NewDAGExecutor creates a DAG executor with the given per-subtask execution function.
func NewDAGExecutor(fn func(ctx context.Context, sub *SubtaskSpec) (string, error)) *DAGExecutor {
	return &DAGExecutor{execFn: fn}
}

// Execute runs all subtasks in dependency order.
// Subtasks with no dependencies (or whose dependencies are met) run in parallel.
// Returns the combined results or the first error encountered.
func (d *DAGExecutor) Execute(ctx context.Context, subtasks []SubtaskSpec) ([]SubtaskSpec, error) {
	if len(subtasks) == 0 {
		return nil, nil
	}

	// Build index.
	byID := make(map[string]int)
	for i := range subtasks {
		byID[subtasks[i].ID] = i
	}

	// Track completion.
	var mu sync.Mutex
	completed := make(map[string]bool)
	results := make([]SubtaskSpec, len(subtasks))
	copy(results, subtasks)

	var firstErr error

	for {
		// Find ready subtasks (all dependencies met, not yet completed).
		var ready []int
		mu.Lock()
		for i, st := range results {
			if completed[st.ID] {
				continue
			}
			if st.Status == TaskStatusFailed {
				continue
			}
			allDepsMet := true
			for _, dep := range st.DependsOn {
				if !completed[dep] {
					allDepsMet = false
					break
				}
			}
			if allDepsMet {
				ready = append(ready, i)
			}
		}
		allDone := len(completed) == len(subtasks)
		mu.Unlock()

		if allDone || len(ready) == 0 {
			break
		}

		// Execute ready subtasks in parallel.
		var wg sync.WaitGroup
		for _, idx := range ready {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				sub := &results[i]
				sub.Status = TaskStatusExecuting

				result, err := d.execFn(ctx, sub)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					sub.Status = TaskStatusFailed
					sub.Result = err.Error()
					if firstErr == nil {
						firstErr = fmt.Errorf("subtask %s: %w", sub.ID, err)
					}
				} else {
					sub.Status = TaskStatusCompleted
					sub.Result = result
				}
				completed[sub.ID] = true
			}(idx)
		}
		wg.Wait()

		if firstErr != nil {
			break
		}
	}

	return results, firstErr
}

// TopologicalOrder returns subtask IDs in valid execution order.
// Returns an error if the graph contains a cycle.
func TopologicalOrder(subtasks []SubtaskSpec) ([]string, error) {
	// Build adjacency and in-degree.
	inDegree := make(map[string]int)
	children := make(map[string][]string)
	ids := make(map[string]bool)

	for _, st := range subtasks {
		ids[st.ID] = true
		if _, ok := inDegree[st.ID]; !ok {
			inDegree[st.ID] = 0
		}
		for _, dep := range st.DependsOn {
			children[dep] = append(children[dep], st.ID)
			inDegree[st.ID]++
		}
	}

	// Kahn's algorithm.
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, child := range children[node] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(order) != len(ids) {
		return nil, fmt.Errorf("cycle detected in subtask dependencies")
	}

	return order, nil
}
