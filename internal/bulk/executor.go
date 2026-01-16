// Package bulk provides bulk operation execution for the Pelican CLI.
package bulk

import (
	"context"
	"fmt"
	"sync"
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
