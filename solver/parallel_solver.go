package solver

import (
	"fmt"
	"sync"

	"github.com/CptPie/DPLL-solver/logger"
	"github.com/CptPie/DPLL-solver/parser"
)

// WorkItem represents a state in the search tree that needs to be explored
type WorkItem struct {
	WorkCopy []*parser.Clause
	Solution *parser.Clause
	Depth    int // Track depth for limiting parallelization
}

// WorkQueue is a thread-safe queue for work items
type WorkQueue struct {
	items  []*WorkItem
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

func NewWorkQueue() *WorkQueue {
	wq := &WorkQueue{
		items: make([]*WorkItem, 0),
	}
	wq.cond = sync.NewCond(&wq.mu)
	return wq
}

func (wq *WorkQueue) Push(item *WorkItem) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	wq.items = append(wq.items, item)
	wq.cond.Signal() // Wake up one waiting worker
}

func (wq *WorkQueue) Pop() *WorkItem {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	// Block while queue is empty and not closed
	for len(wq.items) == 0 && !wq.closed {
		wq.cond.Wait()
	}

	// Return nil if queue is empty (after waking from close)
	if len(wq.items) == 0 {
		return nil
	}

	item := wq.items[len(wq.items)-1]
	wq.items = wq.items[:len(wq.items)-1]
	return item
}

func (wq *WorkQueue) Len() int {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	return len(wq.items)
}

// Close marks the queue as closed and wakes all waiting workers
func (wq *WorkQueue) Close() {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if !wq.closed {
		wq.closed = true
		wq.cond.Broadcast()
	}
}

// WakeAll wakes all waiting workers (e.g., to check termination conditions)
func (wq *WorkQueue) WakeAll() {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	wq.cond.Broadcast()
}

// ParallelSolver manages parallel SAT solving with work stealing
type ParallelSolver struct {
	Problem       *parser.Task
	NumWorkers    int
	ParallelDepth int  // 0 means unlimited, >0 means only parallelize up to this depth
	OptimumMode   bool // If true, find minimal solution instead of stopping at first

	workQueue     *WorkQueue
	resultChan    chan Result
	solutionChan  chan *parser.Clause
	doneChan      chan struct{}
	activeWorkers sync.WaitGroup

	foundSolution    bool
	bestSolution     *parser.Clause
	bestSolutionSize int
	busyWorkers      int
	lastWorkItem     *WorkItem // Last examined work item (useful for UNSAT debugging)
	mu               sync.Mutex

	maxQueueSize int // Maximum work items in queue to prevent memory explosion
}

func NewParallelSolver(task *parser.Task, numWorkers int, parallelDepth int, optimum bool) *ParallelSolver {
	return &ParallelSolver{
		Problem:          task,
		NumWorkers:       numWorkers,
		ParallelDepth:    parallelDepth,
		OptimumMode:      optimum,
		workQueue:        NewWorkQueue(),
		resultChan:       make(chan Result, 1),
		solutionChan:     make(chan *parser.Clause, 1),
		doneChan:         make(chan struct{}),
		maxQueueSize:     numWorkers * 4,     // Limit queue to prevent exponential memory growth
		bestSolutionSize: int(^uint(0) >> 1), // Max int value
	}
}

func (ps *ParallelSolver) HasFoundSolution() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.foundSolution
}

func (ps *ParallelSolver) SetFoundSolution() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.foundSolution = true
}

// UpdateBestSolution updates the best solution if the new one is better (shorter)
// Returns true if this is a better solution
func (ps *ParallelSolver) UpdateBestSolution(solution *parser.Clause) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	solutionSize := len(solution.Vars)
	if solutionSize < ps.bestSolutionSize {
		ps.bestSolution = solution
		ps.bestSolutionSize = solutionSize
		ps.foundSolution = true
		return true
	}
	return false
}

func (ps *ParallelSolver) GetBestSolution() (*parser.Clause, int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.bestSolution, ps.bestSolutionSize
}

func (ps *ParallelSolver) IncrementBusyWorkers() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.busyWorkers++
}

func (ps *ParallelSolver) DecrementBusyWorkers() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.busyWorkers--

	// Check termination conditions while holding the lock
	if ps.busyWorkers == 0 && ps.workQueue.Len() == 0 {
		if ps.OptimumMode || !ps.foundSolution {
			// Close queue to wake all sleeping workers so they can check termination
			ps.workQueue.Close()
		}
	}
}

func (ps *ParallelSolver) GetBusyWorkers() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.busyWorkers
}

func (ps *ParallelSolver) SetLastWorkItem(item *WorkItem) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastWorkItem = item
}

func (ps *ParallelSolver) GetLastWorkItem() *WorkItem {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.lastWorkItem
}

// Solve runs the parallel SAT solver
func (ps *ParallelSolver) Solve() (Result, *parser.Clause) {
	logger.Info("Starting parallel solver with %d workers\n", ps.NumWorkers)

	// Create initial work item
	initialWorkCopy := make([]*parser.Clause, len(ps.Problem.Clauses))
	for i, clause := range ps.Problem.Clauses {
		clauseCopy := &parser.Clause{
			Vars: make([]parser.Variable, len(clause.Vars)),
		}
		copy(clauseCopy.Vars, clause.Vars)
		initialWorkCopy[i] = clauseCopy
	}

	initialItem := &WorkItem{
		WorkCopy: initialWorkCopy,
		Solution: &parser.Clause{},
		Depth:    0,
	}

	ps.workQueue.Push(initialItem)

	// Start worker goroutines
	for i := 0; i < ps.NumWorkers; i++ {
		ps.activeWorkers.Add(1)
		go ps.worker(i)
	}

	// Wait for result
	result := <-ps.resultChan

	// Signal all workers to stop
	close(ps.doneChan)
	ps.workQueue.Close() // Wake up all waiting workers

	// Wait for all workers to finish
	ps.activeWorkers.Wait()

	if result == SATISFIABLE {
		solution := <-ps.solutionChan
		if ps.OptimumMode {
			fmt.Printf("\nOptimal solution found with %d variables: %s\n", len(solution.Vars), solution.String())
		}
		return result, solution
	}

	return result, nil
}

// worker is the main worker goroutine that processes work items
func (ps *ParallelSolver) worker(id int) {
	defer ps.activeWorkers.Done()

	for {
		select {
		case <-ps.doneChan:
			logger.Detail("Worker %d: Received done signal\n", id)
			return
		default:
		}

		// Check if solution was found (only in non-optimum mode)
		if !ps.OptimumMode && ps.HasFoundSolution() {
			logger.Detail("Worker %d: Solution found by another worker, stopping\n", id)
			return
		}

		// Try to get work (blocks if queue is empty)
		item := ps.workQueue.Pop()

		if item == nil {
			// Queue is closed or we woke up to check termination
			// Check if we should send final result
			if ps.workQueue.Len() == 0 && ps.GetBusyWorkers() == 0 {
				logger.Detail("Worker %d: No work available and no busy workers\n", id)

				if ps.OptimumMode && ps.HasFoundSolution() {
					// In optimum mode, we found solution(s), report SATISFIABLE
					select {
					case ps.resultChan <- SATISFIABLE:
						bestSol, bestSize := ps.GetBestSolution()
						logger.Info("Worker %d: Search exhausted. Best solution has %d variables\n", id, bestSize)
						select {
						case ps.solutionChan <- bestSol:
						default:
						}
					default:
						// Result already sent
					}
				} else if !ps.HasFoundSolution() {
					// No solution found at all - problem is UNSAT
					select {
					case ps.resultChan <- UNSATISFIABLE:
						logger.Info("Worker %d: Reporting UNSATISFIABLE\n", id)
					default:
						// Result already sent
					}
				}
			}
			return
		}

		// Mark this worker as busy
		ps.IncrementBusyWorkers()

		// Process this work item
		logger.Detail("Worker %d: Processing work item at depth %d\n", id, item.Depth)
		ps.processWorkItem(item, id)

		// Mark this worker as idle
		ps.DecrementBusyWorkers()
	}
}

// processWorkItem solves a single work item
func (ps *ParallelSolver) processWorkItem(item *WorkItem, workerID int) {
	// Store this as the last examined work item
	ps.SetLastWorkItem(item)

	// Create a solver for this work item
	s := &Solver{
		Problem:         ps.Problem,
		WorkCopy:        item.WorkCopy,
		Solution:        item.Solution,
		Result:          UNKNOWN,
		CheckpointStack: &CheckpointStack{},
	}

	// Run the solving loop
	for {
		// Check if we should stop
		select {
		case <-ps.doneChan:
			return
		default:
		}

		if !ps.OptimumMode && ps.HasFoundSolution() {
			return
		}

		if s.isSolved() {
			if ps.OptimumMode {
				// In optimum mode, check if this is a better solution
				if ps.UpdateBestSolution(s.Solution) {
					bestSol, bestSize := ps.GetBestSolution()
					logger.Info("Worker %d: Found better solution (size %d): %s\n", workerID, bestSize, bestSol.String())
					fmt.Printf("Found solution with %d variables: %s\n", bestSize, bestSol.String())
				}
				// Don't return - try to backtrack and explore other branches
				if s.backtrack() {
					logger.Detail("Worker %d: Backtracking after solution to explore more branches\n", workerID)
					continue
				}
				// No more branches to explore in this work item
				logger.Detail("Worker %d: Exhausted all branches in this work item\n", workerID)
				return
			} else {
				// In normal mode, stop at first solution
				logger.Info("Worker %d: Found solution: %s\n", workerID, s.Solution.String())
				ps.SetFoundSolution()
				select {
				case ps.resultChan <- SATISFIABLE:
				default:
				}
				select {
				case ps.solutionChan <- s.Solution:
				default:
				}
				ps.workQueue.Close() // Wake up all waiting workers
				return
			}
		}

		if s.isUnsolvable() {
			logger.Detail("Worker %d: Branch unsolvable\n", workerID)
			return
		}

		if s.hasContradiction() {
			logger.Detail("Worker %d: Found contradiction, backtracking...\n", workerID)
			if s.backtrack() {
				logger.Detail("Worker %d: Backtracking to previous checkpoint\n", workerID)
				continue
			}
			logger.Detail("Worker %d: No checkpoints left, branch exhausted\n", workerID)
			return
		}

		if s.unitPropagation() {
			logger.Detail("Worker %d: Unit propagation, remaining: %d\n", workerID, len(s.WorkCopy))
			continue
		}

		if s.pureLiteral() {
			logger.Detail("Worker %d: Pure literal, remaining: %d\n", workerID, len(s.WorkCopy))
			continue
		}

		// Handle split - this is where parallelization happens
		if ps.shouldParallelize(item.Depth) {
			// Parallelize this split
			if ps.parallelSplit(s, item.Depth, workerID) {
				logger.Detail("Worker %d: Created parallel split at depth %d\n", workerID, item.Depth)
				continue
			}
		} else {
			// Use sequential split with checkpoints
			if s.split() {
				logger.Detail("Worker %d: Sequential split (depth %d)\n", workerID, item.Depth)
				continue
			}
		}

		if s.backtrack() {
			logger.Detail("Worker %d: Backtracking\n", workerID)
			continue
		}

		logger.Detail("Worker %d: No resolution step found\n", workerID)
		return
	}
}

// shouldParallelize determines if we should create parallel work at this depth
func (ps *ParallelSolver) shouldParallelize(currentDepth int) bool {
	// Check queue size first - don't create more work if queue is full
	if ps.workQueue.Len() >= ps.maxQueueSize {
		return false
	}

	// Check depth limit
	if ps.ParallelDepth == 0 {
		return true // Unlimited depth (but still limited by queue size)
	}
	return currentDepth < ps.ParallelDepth
}

// parallelSplit creates two work items for the split variable
func (ps *ParallelSolver) parallelSplit(s *Solver, currentDepth int, workerID int) bool {
	clauses := s.WorkCopy

	// Find the most used variable (same logic as sequential split)
	variableUsageMap := make(map[int]int)
	for _, clause := range clauses {
		for _, cVar := range clause.Vars {
			if cVar.Impossible {
				continue
			}
			variableUsageMap[cVar.ID] = variableUsageMap[cVar.ID] + 1
		}
	}

	maxVarID := 0
	maxCount := 0
	for variableID, count := range variableUsageMap {
		if count > maxCount {
			maxCount = count
			maxVarID = variableID
		}
	}

	if maxVarID == 0 {
		return false
	}

	// Find the variable
	var pickedVariable *parser.Variable
	for _, clause := range clauses {
		if pickedVariable != nil {
			break
		}
		for _, cVar := range clause.Vars {
			if cVar.ID == maxVarID {
				pickedVariable = &cVar
				break
			}
		}
	}

	if pickedVariable == nil {
		return false
	}

	logger.Detail("Worker %d: Split on variable %s\n", workerID, pickedVariable.String())

	// Create two branches - one with the variable as-is, one with negated
	for _, negated := range []bool{pickedVariable.Negated, !pickedVariable.Negated} {
		splitVar := &parser.Variable{
			ID:         pickedVariable.ID,
			Negated:    negated,
			Impossible: false,
		}

		// Deep copy current state
		newWorkCopy := make([]*parser.Clause, len(s.WorkCopy))
		for i, clause := range s.WorkCopy {
			clauseCopy := &parser.Clause{
				Vars: make([]parser.Variable, len(clause.Vars)),
			}
			copy(clauseCopy.Vars, clause.Vars)
			newWorkCopy[i] = clauseCopy
		}

		newSolution := &parser.Clause{
			Vars: make([]parser.Variable, len(s.Solution.Vars)),
		}
		copy(newSolution.Vars, s.Solution.Vars)
		newSolution.Vars = append(newSolution.Vars, *splitVar)

		// Reduce the working set with this variable
		tmpSolver := &Solver{
			WorkCopy: newWorkCopy,
			Solution: newSolution,
		}
		tmpSolver.reduceWorkingSet(splitVar)

		// Create work item for this branch
		workItem := &WorkItem{
			WorkCopy: tmpSolver.WorkCopy,
			Solution: tmpSolver.Solution,
			Depth:    currentDepth + 1,
		}

		ps.workQueue.Push(workItem)
	}

	// This worker is done with this branch - work items pushed to queue
	return true
}
