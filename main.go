package main

import (
	"fmt"
	"os"

	"github.com/CptPie/DLPP-solver/parser"
	"github.com/CptPie/DLPP-solver/solver"
	"github.com/CptPie/DLPP-solver/utils"
	"github.com/alexflint/go-arg"
)

var Args struct {
	File string `arg:"required,positional" help:"Path to the input file, in DIMACS format"`
}

func main() {
	// read cli argument
	arg.MustParse(&Args)

	// create parser object
	parser, err := parser.NewParser(Args.File)
	if err != nil {
		fmt.Printf("Parser error: %v\n", err)
		os.Exit(1)
	}

	// parse input file
	task, err := parser.Parse()
	if err != nil {
		fmt.Printf("Parser error: %v\n", err)
		os.Exit(1)
	}

	// write debug file showing the parser output
	f, err := os.Create("parser.out")
	if err != nil {
		fmt.Printf("Could not create parser output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	_, err = f.WriteString(utils.JSONString(task))
	if err != nil {
		fmt.Printf("Could not create parser output file: %v\n", err)
		os.Exit(1)
	}

	// Verify DIMACS compliance
	err = task.Verify()
	if err != nil {
		fmt.Printf("Parsing result is not valid: %v\n", err)
		os.Exit(1)
	}

	// Solve
	solver := solver.NewSolver(task)
	solver.Solve()
}
