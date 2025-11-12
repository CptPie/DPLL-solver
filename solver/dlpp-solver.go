package solver

import (
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
	Result   Result           // Solver result status
	Problem  *parser.Task     // The problem to solve
	WorkCopy []*parser.Clause // Working copy of the clauses (used for reducing)
	Solution *parser.Clause   // The found solution
}

func NewSolver(task *parser.Task) *Solver {

	sol := &parser.Clause{}

	return &Solver{
		Problem:  task,
		WorkCopy: task.Clauses,
		Result:   UNKNOWN,
		Solution: sol,
	}
}

func (s *Solver) Solve() {
	step := 0
	timeout := 0
	// while true
	for {
		if timeout >= 10000 {
			utils.JSONPrint(s)
			break
		}

		if s.isSolved() {
			s.Result = SATISFIABLE
			break
		}

		if s.isUnsolvable() {
			s.Result = UNSATISFIABLE
			break
		}

		if s.unitPropagation() {
			step++
		}

		timeout++
	}
}

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
