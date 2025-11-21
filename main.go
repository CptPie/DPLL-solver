package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/CptPie/DLPP-solver/logger"
	"github.com/CptPie/DLPP-solver/parser"
	"github.com/CptPie/DLPP-solver/solver"
	"github.com/CptPie/DLPP-solver/utils"
	"github.com/alexflint/go-arg"
)

var Args struct {
	File            string `arg:"required,positional" help:"Path to the input file, in DIMACS format"`
	LogLevel        string `arg:"--log-level,-l" default:"none" help:"Log level: 'none', 'steps', or 'full' (default: none)"`
	Parallel        bool   `arg:"--parallel,-p" help:"Enable parallel solving"`
	Threads         int    `arg:"--threads,-t" help:"Number of worker threads (default: half of available CPUs, requires --parallel)"`
	ParallelDepth   int    `arg:"--parallel-depth,-d" default:"0" help:"Only parallelize splits up to this depth (0 = unlimited, requires --parallel)"`
	Optimum         bool   `arg:"--optimum,-o" help:"Find minimal solution (fewest variable assignments, requires --parallel)"`
}

func main() {
	// read cli argument
	arg.MustParse(&Args)

	// Set log level
	logger.SetLevel(logger.ParseLevel(Args.LogLevel))

	// Check if parallel mode is enabled
	if !Args.Parallel {
		// Warn if user specified parallel-only flags
		if Args.Threads > 0 {
			fmt.Println("Warning: --threads requires --parallel flag, ignoring")
		}
		if Args.ParallelDepth > 0 {
			fmt.Println("Warning: --parallel-depth requires --parallel flag, ignoring")
		}
		if Args.Optimum {
			fmt.Println("Warning: --optimum requires --parallel flag, ignoring")
		}
	} else {
		// Set thread count - default to half of available CPUs if not specified
		if Args.Threads == 0 {
			Args.Threads = runtime.NumCPU() / 2
			if Args.Threads < 1 {
				Args.Threads = 1
			}
		}
		logger.Info("Using %d worker threads\n", Args.Threads)
		if Args.ParallelDepth > 0 {
			logger.Info("Parallelizing only up to depth %d\n", Args.ParallelDepth)
		}
	}

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
	if Args.Parallel {
		// Use parallel solver
		parallelSolver := solver.NewParallelSolver(task, Args.Threads, Args.ParallelDepth, Args.Optimum)
		result, solution := parallelSolver.Solve()
		logger.Info("Result: %s\n", result.String())
		if result == solver.SATISFIABLE {
			logger.Info("Solution: %s\n", solution.String())
		}
	} else {
		// Use sequential solver
		sequentialSolver := solver.NewSolver(task)
		sequentialSolver.Solve()
		logger.Info("Result: %s\n", sequentialSolver.Result.String())
		if sequentialSolver.Result == solver.SATISFIABLE {
			logger.Info("Solution: %s\n", sequentialSolver.Solution.String())
		}
	}
}
