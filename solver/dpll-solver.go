package solver

import (
	"github.com/CptPie/DPLL-solver/logger"
	"github.com/CptPie/DPLL-solver/parser"
	"github.com/CptPie/DPLL-solver/utils"
)

type Result int

const (
	UNSOLVED      Result = iota // 0
	SATISFIABLE                 // 1
	UNSATISFIABLE               // 2
	UNKNOWN                     // 3
)

func (r Result) String() string {
	return [...]string{"UNSOLVED", "SATISFIABLE", "UNSATISFIABLE", "UNKNOWN"}[r]
}

type Solver struct {
	Result          Result           // Solver result status
	Problem         *parser.Task     // The problem to solve
	WorkCopy        []*parser.Clause // Working copy of the clauses (used for reducing)
	Solution        *parser.Clause   // The found solution
	CheckpointStack *CheckpointStack // Stack for storing checkpoints for backtracking
}

type Checkpoint struct {
	WorkCopy []*parser.Clause
	Solution *parser.Clause
}

type CheckpointStack struct {
	checkpoints []*Checkpoint
	count       int
}

func (cs *CheckpointStack) Push(c *Checkpoint) {
	cs.checkpoints = append(cs.checkpoints[:cs.count], c)
	cs.count++
}

func (cs *CheckpointStack) Pop() *Checkpoint {
	if cs.count == 0 {
		return nil
	}
	cs.count = cs.count - 1
	res := cs.checkpoints[cs.count]
	cs.checkpoints = cs.checkpoints[:cs.count]
	return res
}

func NewSolver(task *parser.Task) *Solver {
	sol := &parser.Clause{}

	// Create a deep copy of the task's clauses for WorkCopy
	// to avoid modifying the original task data
	workCopy := make([]*parser.Clause, len(task.Clauses))
	for i, clause := range task.Clauses {
		clauseCopy := &parser.Clause{
			Vars: make([]parser.Variable, len(clause.Vars)),
		}
		copy(clauseCopy.Vars, clause.Vars)
		workCopy[i] = clauseCopy
	}

	return &Solver{
		Problem:         task,
		WorkCopy:        workCopy,
		Result:          UNKNOWN,
		Solution:        sol,
		CheckpointStack: &CheckpointStack{},
	}
}

func (s *Solver) markCheckpoint() *Checkpoint {
	// Deep copy the WorkCopy - must copy the clauses themselves, not just the slice of pointers
	wc := make([]*parser.Clause, len(s.WorkCopy))
	for i, clause := range s.WorkCopy {
		// Create a new clause
		clauseCopy := &parser.Clause{
			Vars: make([]parser.Variable, len(clause.Vars)),
		}
		// Deep copy all variables in the clause
		copy(clauseCopy.Vars, clause.Vars)
		wc[i] = clauseCopy
	}

	solutionCopy := parser.Clause{}
	for _, cvar := range s.Solution.Vars {
		solutionCopy.Vars = append(solutionCopy.Vars, cvar)
	}

	return &Checkpoint{
		WorkCopy: wc,
		Solution: &solutionCopy,
	}
}

func (s *Solver) Solve() {
	logger.Info("Starting to solve %d clauses.\n", len(s.WorkCopy))
	logger.Detail("%s\n", s.WorkCopy)

	// Counters for tracking which steps are executed
	unitPropCount := 0
	pureLiteralCount := 0
	splitCount := 0
	contradictionBacktrackCount := 0
	fallbackBacktrackCount := 0

	// while true
	for {
		if s.isSolved() {
			//logger.Info("Found solution: %s\n", s.Solution.String())
			s.Result = SATISFIABLE
			break
		}

		if s.isUnsolvable() {
			//logger.Info("Problem is unsolvable.\n")
			//logger.Detail("Solution: %s\n Remaining clauses:%s\n", utils.JSONString(s.Solution), utils.JSONString(s.WorkCopy))
			s.Result = UNSATISFIABLE
			break
		}

		// Check for contradictions: if any clause has all variables marked as impossible, we need to backtrack
		if s.hasContradiction() {
			logger.Step("Found contradiction, backtracking...\n")
			if s.backtrack() {
				contradictionBacktrackCount++
				logger.Step("Backtracking to previous checkpoint, remaining clauses: %d\n", len(s.WorkCopy))
				logger.Detail("%s\n", s.WorkCopy)
				continue
			}
			// No checkpoints left, problem is unsolvable
			logger.Info("Problem is unsolvable.\n")
			logger.Detail("Solution: %s\n Remaining clauses:%s\n", utils.JSONString(s.Solution), utils.JSONString(s.WorkCopy))
			s.Result = UNSATISFIABLE
			break
		}

		if s.unitPropagation() {
			unitPropCount++
			logger.Step("Found a unit propagation, remaining clauses to solve: %d\n", len(s.WorkCopy))
			logger.Detail("%s\n", s.WorkCopy)
			continue
		}

		if s.pureLiteral() {
			pureLiteralCount++
			logger.Step("Found a pure literal, remaining clauses to solve: %d\n", len(s.WorkCopy))
			logger.Detail("%s\n", s.WorkCopy)
			continue
		}

		if s.split() {
			splitCount++
			logger.Step("Found a split, remembering checkpoint, remaining clauses to solve: %d\n", len(s.WorkCopy))
			logger.Detail("%s\n", s.WorkCopy)
			continue
		}

		if s.backtrack() {
			fallbackBacktrackCount++
			logger.Step("Backtracking to previous checkpoint, remaining clauses: %d\n", len(s.WorkCopy))
			logger.Detail("%s\n", s.WorkCopy)
			continue
		}

		logger.Step("No resolution step found\n")
		// TODO backtrack here i guess?
		// otherwise UNSATISFIABLE
		logger.Detail("%s\n", s.WorkCopy)

		break
	}

	// Print step execution summary
	logger.Info("=== DPLL Step Execution Summary ===\n")
	logger.Info("Unit Propagation:        %d times\n", unitPropCount)
	logger.Info("Pure Literal:            %d times\n", pureLiteralCount)
	logger.Info("Split:                   %d times\n", splitCount)
	logger.Info("Contradiction Backtrack: %d times\n", contradictionBacktrackCount)
	logger.Info("Fallback Backtrack:      %d times\n", fallbackBacktrackCount)
	logger.Info("===================================\n")
}

func (s *Solver) isSolved() bool {
	return len(s.WorkCopy) == 0
}

// hasContradiction checks if any clause has all its variables marked as impossible
// This indicates we've reached a dead end and need to backtrack
func (s *Solver) hasContradiction() bool {
	for _, clause := range s.WorkCopy {
		allImpossible := true
		for _, cVar := range clause.Vars {
			if !cVar.Impossible {
				allImpossible = false
				break
			}
		}
		if allImpossible && len(clause.Vars) > 0 {
			return true
		}
	}
	return false
}

func (s *Solver) isUnsolvable() bool {
	if len(s.WorkCopy) == 0 {
		return false
	}

	if s.CheckpointStack.count != 0 {
		return false
	}

	for _, clause := range s.WorkCopy {
		for _, cVar := range clause.Vars {
			if !cVar.Impossible {
				return false
			}
		}
	}
	return true
}

// This function implements unitPropagation, returns a boolean value representing work being done (an successful reduction)
func (s *Solver) unitPropagation() bool {
	clauses := s.WorkCopy

	for clauseID, clause := range clauses {
		// Count non-impossible variables to find unit clauses
		nonImpossibleCount := 0
		var unit parser.Variable
		for _, v := range clause.Vars {
			if !v.Impossible {
				nonImpossibleCount++
				unit = v
			}
		}

		if nonImpossibleCount == 1 {
			// We found a single variable clause -> Add it to the solution.
			s.Solution.Vars = append(s.Solution.Vars, unit)

			// Remove it from the working set
			clauses = append(clauses[:clauseID], clauses[clauseID+1:]...)

		preLoop:
			// Now we need to find other clauses, containing this variable in this state and remove them from the working set
			for otherClauseID, otherClause := range clauses {
				for otherClauseVarID, otherClauseVar := range otherClause.Vars {
					if otherClauseVar.ID == unit.ID {
						if otherClauseVar.Negated == unit.Negated {
							// this is the same Variable we just found through unit propagation with the same state. Remove the clause from the set.
							clauses = append(clauses[:otherClauseID], clauses[otherClauseID+1:]...)
							// we updated clauses mid loop, restart the iteration
							goto preLoop
						} else {
							// this is the same Variable, but the opposite state. Mark it as impossible and update the clause in the working set.
							otherClauseVar.Impossible = true
							otherClause.Vars[otherClauseVarID] = otherClauseVar
							clauses[otherClauseID] = otherClause
						}
					}
				}
			}

			// Update WorkCopy and return true - we found and processed a unit clause
			// Return early to avoid index issues, function will be called again if needed
			s.WorkCopy = clauses
			return true
		}
	}
	return false
}

func (s *Solver) pureLiteral() bool {
	didWork := false

	clauses := s.WorkCopy

	// Track which variables appear in which polarities
	// Key: variable ID, Value: map[negated]bool (true if that polarity has been seen)
	variablePolarity := make(map[int]map[bool]bool)

	// Scan all clauses to find polarities
	for _, clause := range clauses {
		for _, cVar := range clause.Vars {
			if cVar.Impossible {
				continue // Skip impossible variables
			}
			if _, ok := variablePolarity[cVar.ID]; !ok {
				variablePolarity[cVar.ID] = make(map[bool]bool)
			}
			variablePolarity[cVar.ID][cVar.Negated] = true
		}
	}

	// Find pure literals (variables that appear in only one polarity)
	pureLiterals := make(map[int]parser.Variable)
	for varID, polarities := range variablePolarity {
		if len(polarities) == 1 {
			// This variable appears in only one polarity, it's a pure literal
			// Find the actual variable to add to solution
			for _, clause := range clauses {
				for _, cVar := range clause.Vars {
					if cVar.ID == varID && !cVar.Impossible {
						pureLiterals[varID] = cVar
						break
					}
				}
				if _, found := pureLiterals[varID]; found {
					break
				}
			}
		}
	}

	// Remove clauses containing pure literals and add them to solution
	if len(pureLiterals) > 0 {
		for _, pureLit := range pureLiterals {
			s.Solution.Vars = append(s.Solution.Vars, pureLit)

			// Remove all clauses containing this pure literal
			newClauses := make([]*parser.Clause, 0)
			for _, clause := range clauses {
				containsPureLit := false
				for _, cVar := range clause.Vars {
					if cVar.ID == pureLit.ID && cVar.Negated == pureLit.Negated && !cVar.Impossible {
						containsPureLit = true
						break
					}
				}
				if !containsPureLit {
					newClauses = append(newClauses, clause)
				}
			}
			clauses = newClauses
			didWork = true
		}
	}

	s.WorkCopy = clauses
	return didWork
}

func (s *Solver) split() bool {
	clauses := s.WorkCopy
	didWork := false
	// At the point where split is even able to be called, there should be no "free"/"easy" variables to resolve.
	// Count the appearances of non impossible variables and pick one with the highest count.

	var pickedVariable *parser.Variable

	// prepare a map to count each variable label (IDs) appearances
	variableUsageMap := make(map[int]int)

	// count appearances
	for _, clause := range clauses {
		for _, cVar := range clause.Vars {
			// ignore impossible Vars
			if cVar.Impossible {
				continue
			}
			// check if it exists in the usage map
			_, ok := variableUsageMap[cVar.ID]
			if ok {
				// increment usage
				variableUsageMap[cVar.ID] = variableUsageMap[cVar.ID] + 1
			} else {
				// add it to the map
				variableUsageMap[cVar.ID] = 1
			}
		}
	}

	// find the variable with largest count (by ID)
	maxVarID := 0
	maxCount := 0

	for variableID, count := range variableUsageMap {
		if count > maxCount {
			maxCount = count
			maxVarID = variableID
		}
	}

	if maxVarID == 0 {
		// we found nothing?! wtf?!
		return didWork
	}

	// find the first variable occurance matching the ID we just detected
	for _, clause := range clauses {
		if pickedVariable != nil {
			break
		}
		for _, cVar := range clause.Vars {
			if cVar.ID == maxVarID {
				// found it, pick it
				pickedVariable = &cVar

				logger.Detail("Found a split candidate: %s\n", cVar.String())

				// remember this for the checkpoint, pick the opposite state in the checkpoint
				checkpoint := s.markCheckpoint()

				checkpointVar := &parser.Variable{
					ID:         cVar.ID,
					Impossible: cVar.Impossible,
				}
				if cVar.Negated {
					checkpointVar.Negated = false
				} else {
					checkpointVar.Negated = true
				}

				logger.Detail("CheckpointVar: %s\n", checkpointVar.String())

				checkpoint.Solution.Vars = append(checkpoint.Solution.Vars, *checkpointVar)
				s.CheckpointStack.Push(checkpoint)

				// add it to the current solution
				s.Solution.Vars = append(s.Solution.Vars, cVar)
				logger.Detail("checkpoint Solution %s\n", checkpoint.Solution)
				logger.Detail("solver solution: %s\n", s.Solution)
				break
			}
		}
	}

	didWork = s.reduceWorkingSet(pickedVariable)

	return didWork
}

func (s *Solver) backtrack() bool {
	if s.CheckpointStack.count == 0 {
		logger.Detail("No more checkpoints to backtrack to\n")
		return false
	}

	logger.Detail("CPS Pre backtrack: %v\n", s.CheckpointStack)
	logger.Detail("WorkCopy: %s\n", s.WorkCopy)
	logger.Detail("Solution: %s\n", s.Solution)

	stack := *s.CheckpointStack
	backtrackPoint := *stack.Pop()

	logger.Detail("%v\n", backtrackPoint)

	s.CheckpointStack = &stack

	sol := *backtrackPoint.Solution

	// Create a new slice to avoid aliasing with the checkpoint's WorkCopy
	restoredWorkCopy := make([]*parser.Clause, len(backtrackPoint.WorkCopy))
	copy(restoredWorkCopy, backtrackPoint.WorkCopy)
	s.WorkCopy = restoredWorkCopy
	s.Solution = &sol

	logger.Detail("CPS Post backtrack: %v\n", s.CheckpointStack)
	logger.Detail("WorkCopy: %s\n", s.WorkCopy)
	logger.Detail("Solution: %s\n", s.Solution)

	// reduce with the last variabele (the variable that caused the split in the first place)
	s.reduceWorkingSet(&sol.Vars[len(sol.Vars)-1])

	return true
}

func (s *Solver) reduceWorkingSet(rVar *parser.Variable) bool {
	clauses := s.WorkCopy
	didWork := false
preLoop:
	// Remove clauses with this variable state (they are solved), mark opposite state as impossible
	for clauseID, clause := range clauses {
		for cVarID, cVar := range clause.Vars {
			if cVar.Impossible {
				continue
			}
			if cVar.ID == rVar.ID {
				if cVar.Negated == rVar.Negated {
					// clause contains variable with the same negation state, remove the entire clause as it is solved
					logger.Detail("Clause %s (ID: %d) contains variable %s, removing...\n", clause, clauseID, rVar)
					clauses = append(clauses[:clauseID], clauses[clauseID+1:]...)
					goto preLoop
				} else {
					cVar.Impossible = true
					clause.Vars[cVarID] = cVar
					clauses[clauseID] = clause
				}
				didWork = true
			}
		}
	}
	s.WorkCopy = clauses
	return didWork
}
