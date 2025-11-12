package main

import (
	"fmt"
	"os"

	"github.com/CptPie/DLPP-solver/parser"
	"github.com/CptPie/DLPP-solver/solver"
	"github.com/alexflint/go-arg"
)

var Args struct {
	File string `arg:"required,positional" help:"Path to the input file, in DIMACS format"`
}

func main() {
	arg.MustParse(&Args)

	parser, err := parser.NewParser(Args.File)
	if err != nil {
		fmt.Printf("Parser error: %v\n", err)
		os.Exit(1)
	}

	task, err := parser.Parse()
	if err != nil {
		fmt.Printf("Parser error: %v\n", err)
		os.Exit(1)
	}

	err = task.Verify()
	if err != nil {
		fmt.Printf("Parsing result is not valid: %v\n", err)
		os.Exit(1)
	}

	solver := solver.NewSolver(task)

	solver.Solve()

}
