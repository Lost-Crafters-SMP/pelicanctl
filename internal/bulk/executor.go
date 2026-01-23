// Package bulk provides bulk operation execution for pelicanctl.
package bulk

import (
	"context"
	"fmt"
	"sync"

	"go.lostcrafters.com/pelicanctl/internal/output"
)

// Operation represents a single operation to execute.
type Operation struct {
	ID   string
	Name string
	Exec func() error
}

// Result represents the result of an operation.
type Result struct {
	Operation Operation
	Success   bool
	Error     error
}

// Executor executes operations in parallel.
type Executor struct {
	maxConcurrency  int
	continueOnError bool
	failFast        bool
}

// NewExecutor creates a new bulk executor.
func NewExecutor(maxConcurrency int, continueOnError bool, failFast bool) *Executor {
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // Default
	}
	return &Executor{
		maxConcurrency:  maxConcurrency,
		continueOnError: continueOnError,
		failFast:        failFast,
	}
}

// Execute executes a list of operations in parallel.
func (e *Executor) Execute(_ context.Context, operations []Operation) []Result {
	results := make([]Result, len(operations))

	// Semaphore for limiting concurrency
	sem := make(chan struct{}, e.maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var hasError bool

	for i, op := range operations {
		// Check if we should fail fast
		if e.failFast && hasError {
			// Mark remaining operations as not executed
			for j := i; j < len(operations); j++ {
				results[j] = Result{
					Operation: operations[j],
					Success:   false,
					Error:     fmt.Errorf("skipped due to previous error"), //nolint:perfsprint // Error message
				}
			}
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(idx int, operation Operation) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			result := Result{
				Operation: operation,
			}

			// Execute operation
			if err := operation.Exec(); err != nil {
				result.Success = false
				result.Error = err
				mu.Lock()
				hasError = true
				mu.Unlock()
			} else {
				result.Success = true
			}

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, op)
	}

	wg.Wait()
	return results
}

// Summary returns a summary of results.
type Summary struct {
	Total   int
	Success int
	Failed  int
	Results []Result
}

// GetSummary returns a summary of the execution results.
func GetSummary(results []Result) Summary {
	summary := Summary{
		Total:   len(results),
		Results: results,
	}

	for _, result := range results {
		if result.Success {
			summary.Success++
		} else {
			summary.Failed++
		}
	}

	return summary
}

// PrintBulkJSON prints bulk operation results in minimal JSON format.
// Each result contains only server_identifier, status ("success" | "error"), and optional error.
func PrintBulkJSON(formatter *output.Formatter, results []Result, summary Summary, continueOnError bool) error {
	outputData := make([]map[string]any, 0, len(results))

	for _, result := range results {
		resultData := map[string]any{
			"server_identifier": result.Operation.ID,
		}
		if result.Success {
			resultData["status"] = "success"
		} else {
			resultData["status"] = "error"
			resultData["error"] = result.Error.Error()
		}
		outputData = append(outputData, resultData)
	}

	response := map[string]any{
		"results": outputData,
		"summary": map[string]any{
			"succeeded": summary.Success,
			"failed":    summary.Failed,
		},
	}

	if err := formatter.Print(response); err != nil {
		return err
	}

	if summary.Failed > 0 && !continueOnError {
		return fmt.Errorf("%d operation(s) failed", summary.Failed)
	}

	return nil
}
