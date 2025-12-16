package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/CptPie/DPLL-solver/logger"
	dimacsParser "github.com/CptPie/DPLL-solver/parser"
	"github.com/CptPie/DPLL-solver/solver"
	"github.com/CptPie/DPLL-solver/utils"
	"github.com/alexflint/go-arg"
)

var Args struct {
	File          string `arg:"--file,-f" help:"Path to the input file or folder of input files, in DIMACS format"`
	LogLevel      string `arg:"--log-level,-l" default:"none" help:"Log level: 'none', 'steps', or 'full' (default: none)"`
	Parallel      bool   `arg:"--parallel,-p" help:"Enable parallel solving"`
	Threads       int    `arg:"--threads,-t" help:"Number of worker threads (default: half of available CPUs, requires --parallel)"`
	ParallelDepth int    `arg:"--parallel-depth,-d" default:"0" help:"Only parallelize splits up to this depth (0 = unlimited, requires --parallel)"`
	Optimum       bool   `arg:"--optimum,-o" help:"Find minimal solution (fewest variable assignments, requires --parallel)"`
	NumFiles      int    `arg:"--NumFiles,-n" default: "-1" help:"Number of files to be solved in case of 'File' being a folder (default: all Files)"`
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

	if Args.File != "" {
		fileInfo, err := os.Stat(Args.File)
		if err != nil {
			fmt.Printf("Failed to open path: %s, no such file or directory\n", Args.File)
			os.Exit(1)
		}
		if fileInfo.IsDir() {
			dir, err := os.Open(Args.File)
			if err != nil {
				fmt.Printf("Failed to open path: %s, no such file or directory\n", Args.File)
				os.Exit(1)
			}

			var numFiles int
			if Args.NumFiles <= 0 {
				numFiles = -1 // unlimited
			} else {
				numFiles = Args.NumFiles
			}

			files, err := dir.Readdir(numFiles)
			fmt.Printf("Folder solve mode, solving %d files\n", len(files))
			startTime := time.Now()
			i := 0
			for _, f := range files {
				i++
				analyzeFile(Args.File + f.Name())
				fmt.Printf("%d/%d done\n\n", i, len(files))
			}
			endTime := time.Now()
			dur := endTime.Sub(startTime)
			avg := dur / time.Duration(len(files))

			fmt.Printf("Solving of %d files took %v; Average: %v\n", len(files), dur, avg)
		} else {
			analyzeFile(Args.File)
		}
	} else {

		scanner := bufio.NewScanner(os.Stdin)

		lines := []string{}
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
		}

		analyzeInput(lines)
	}
}

func analyzeFile(fileName string) {
	if fileName == "" {
		fmt.Errorf("could not create parser, no file given")
		os.Exit(1)
	}
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Errorf("failed to open file: %v", err)
		os.Exit(1)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Err() != nil {
			fmt.Errorf("failed to read file: %v", scanner.Err())
			os.Exit(1)
		}
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	analyze(fileName, lines)
}

func analyzeInput(lines []string) {
	analyze("CLI", lines)
}

func analyze(fileName string, lines []string) {

	fmt.Printf("Analyzing file %s\n", fileName)
	startTime := time.Now()

	// create parser object
	parser, err := dimacsParser.NewParser(fileName, lines)
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

	var result solver.Result
	var solution *dimacsParser.Clause
	var workCopy []*dimacsParser.Clause

	// Solve
	if Args.Parallel {
		// Use parallel solver
		parallelSolver := solver.NewParallelSolver(task, Args.Threads, Args.ParallelDepth, Args.Optimum)
		result, solution = parallelSolver.Solve()

		// Get last examined work item for UNSAT debugging
		lastWorkItem := parallelSolver.GetLastWorkItem()
		if lastWorkItem != nil {
			workCopy = lastWorkItem.WorkCopy
			// For UNSAT cases, use the last examined solution instead of nil
			if result == solver.UNSATISFIABLE && lastWorkItem.Solution != nil {
				solution = lastWorkItem.Solution
			}
		}
	} else {
		// Use sequential solver
		sequentialSolver := solver.NewSolver(task)
		sequentialSolver.Solve()
		workCopy = sequentialSolver.WorkCopy
		result = sequentialSolver.Result
		solution = sequentialSolver.Solution

	}
	endTime := time.Now()
	logger.Info("Finished analysis. Problem is %s ", result)
	if result == solver.SATISFIABLE {
		logger.Info(" Found solution: %s\n", solution)
		logger.Info("s %s\n", solver.SATISFIABLE)

		sort.Slice(solution.Vars, func(i, j int) bool {
			return abs(solution.Vars[i].ID) < abs(solution.Vars[j].ID)
		})

		builder := new(strings.Builder)
		builder.WriteString("v ")
		for _, elem := range solution.Vars {
			builder.WriteString(elem.CleanString())
			builder.WriteString(" ")
		}

		builder.WriteString("0\n")
		logger.Info(builder.String())

	} else if result == solver.UNSATISFIABLE {
		logger.Info(" Last examined solution: %s\nOpen clauses to solve: %s\n", solution, workCopy)
		logger.Info("s %s\n", solver.UNSATISFIABLE)

		builder := new(strings.Builder)
		builder.WriteString("v ")
		for _, elem := range solution.Vars {
			builder.WriteString(elem.CleanString())
			builder.WriteString(" ")
		}

		builder.WriteString("0\n")
		logger.Info(builder.String())

	}
	logger.Info("Time elapsed: %v\n", endTime.Sub(startTime))

}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
