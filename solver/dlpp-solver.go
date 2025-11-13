package solver

import (
	"fmt"

	"github.com/CptPie/DLPP-solver/parser"
	"github.com/CptPie/DLPP-solver/utils"
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
	cs.count--
	return cs.checkpoints[cs.count]
}

func NewSolver(task *parser.Task) *Solver {

	sol := &parser.Clause{}

	return &Solver{
		Problem:         task,
		WorkCopy:        task.Clauses,
		Result:          UNKNOWN,
		Solution:        sol,
		CheckpointStack: &CheckpointStack{},
	}
}

func (s *Solver) markCheckpoint() *Checkpoint {
	wc := make([]*parser.Clause, len(s.WorkCopy))
	copy(wc, s.WorkCopy)
	sol := *s.Solution
	return &Checkpoint{
		WorkCopy: wc,
		Solution: &sol,
	}
}

func (s *Solver) Solve() {
	fmt.Printf("Starting to solve %d clauses.\n", len(s.WorkCopy))
	fmt.Println(s.WorkCopy)
	// while true
	for {
		if s.isSolved() {
			fmt.Printf("Found solution: %s\n", s.Solution.String())
			s.Result = SATISFIABLE
			break
		}

		if s.isUnsolvable() {
			fmt.Printf("Problem is unsolvable.\nSolution: %s\n Remaining clauses:%s\n", utils.JSONString(s.Solution), utils.JSONString(s.WorkCopy))
			s.Result = UNSATISFIABLE
			break
		}

		if s.unitPropagation() {
			fmt.Printf("Found a unit propagation, remaining clauses to solve: %d\n", len(s.WorkCopy))
			fmt.Println(s.WorkCopy)
			continue
		}

		if s.pureLiteral() {
			fmt.Printf("Found a pure literal, remaining clauses to solve: %d\n", len(s.WorkCopy))
			fmt.Println(s.WorkCopy)
			continue
		}

		if s.split() {
			fmt.Printf("Found a split, remembering checkpoint, remaining clauses to solve: %d\n", len(s.WorkCopy))
			fmt.Println(s.WorkCopy)
			continue
		}

		fmt.Println("No resolution step found")
		fmt.Println(s.WorkCopy)

		break
	}
}

func (s *Solver) isSolved() bool {
	if len(s.WorkCopy) == 0 {
		return true
	}

	return false
}

func (s *Solver) isUnsolvable() bool {
	if len(s.WorkCopy) == 0 {
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

	didWork := false

	for clauseID, clause := range clauses {
		if len(clause.Vars) == 1 {
			unit := clause.Vars[0]
			// We found a single variable clause -> Add it to the solution.
			s.Solution.Vars = append(s.Solution.Vars, unit)

			// Remove it from the working set
			clauses = append(clauses[:clauseID], clauses[clauseID+1:]...)

			didWork = true

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
		}
	}
	s.WorkCopy = clauses
	return didWork
}

func (s *Solver) pureLiteral() bool {
	didWork := false

	clauses := s.WorkCopy

	// prepare a map to count each variable label (IDs) appearances
	variableUsageMap := make(map[int]int)

	// count appearances
	for _, clause := range clauses {
		for _, cVar := range clause.Vars {
			_, ok := variableUsageMap[cVar.ID]
			if ok {
				variableUsageMap[cVar.ID]++
			} else {
				variableUsageMap[cVar.ID] = 1
			}
		}
	}

	// check appearances for variables only used once
	for cVarID, count := range variableUsageMap {
		if count == 1 {
			// variable "cVarID" is only used once, lets find it in the clauses (should only ever get one matching clause)
			for clauseID, clause := range clauses {
				for _, cVar := range clause.Vars {
					if cVar.ID == cVarID {
						// if cVar is impossible to solve (due to a previous step) ignore it for pure Literal reduction
						if !cVar.Impossible {
							// found the variable, add it to the solution, remove the clause from the workset
							s.Solution.Vars = append(s.Solution.Vars, cVar)
							clauses = append(clauses[:clauseID], clauses[clauseID+1:]...)
							didWork = true
						}
					}
				}
			}
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

	fmt.Println(variableUsageMap)

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

				fmt.Printf("Found a split candidate: %s\n", cVar.String())

				// remember this for the checkpoint, pick the opposite state in the checkpoint
				checkpoint := s.markCheckpoint()
				checkpointVar := &parser.Variable{
					ID:         cVar.ID,
					Negated:    !cVar.Negated,
					Impossible: cVar.Impossible,
				}
				checkpoint.Solution.Vars = append(checkpoint.Solution.Vars, *checkpointVar)
				s.CheckpointStack.Push(checkpoint)

				// add it to the current solution
				s.Solution.Vars = append(s.Solution.Vars, cVar)
				break
			}
		}
	}

preLoop:
	// Remove clauses with this variable state (they are solved), mark opposite state as impossible
	for clauseID, clause := range clauses {
		for cVarID, cVar := range clause.Vars {
			if cVar.Impossible {
				continue
			}
			if cVar.ID == pickedVariable.ID {
				if cVar.Negated == pickedVariable.Negated {
					// clause contains variable with the same negation state, remove the entire clause as it is solved
					fmt.Printf("Clause %s (ID: %d) contains variable %s, removing...\n", clause, clauseID, pickedVariable)
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
