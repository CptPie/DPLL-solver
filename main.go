package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/CptPie/DLPP-solver/parser"
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

	jsonData, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))

}
