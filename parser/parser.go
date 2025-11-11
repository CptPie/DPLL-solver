package parser

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Format to parse:
// 0 to n lines of comments, starting with 'c'
//
// ##### STANDARD
// 1 line of an instance prompt of the form: p {name} {nvar} {nbclauses}
// 		- {name} is the name of the prompt
//    - {nvar} is the exact number of variables in the prompt
//    - {nbclauses} is the exact number of clauses contained
//
// Example:
// c
// c start with comments
// c
// c
// p cnf 5 3
// 1 -5 4 0
// -1 5 3 4 0
// -3 -4 0
//
//
// ##### GROUPED
// 1 line of an instance prompt of the form: p {name} {nvar} {nbclauses} {lastgroupindex}
// 		- {name} is the name of the prompt
//    - {nvar} is the exact number of variables in the prompt
//    - {nbclauses} is the exact number of clauses contained

type Parser struct {
	FilePath string
	Lines    []string
}

type Task struct {
	Name       string
	NumVars    int
	NumClauses int
	Clauses    []*Clause
}

type Clause struct {
	Vars []Variable
}

type Variable struct {
	ID      int
	Negated bool
}

func NewParser(filepath string) (*Parser, error) {
	if filepath == "" {
		return nil, fmt.Errorf("could not create parser, no file given")
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Err() != nil {
			return nil, fmt.Errorf("failed to read file: %v", scanner.Err())
		}
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	return &Parser{
		FilePath: filepath,
		Lines:    lines,
	}, nil
}

func (p *Parser) Parse() (*Task, error) {
	clauses := []*Clause{}
	task := &Task{}
	for _, line := range p.Lines {
		// Empty line
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// Comment line
		parts := strings.Split(trimmedLine, " ")
		if parts[0] == "c" || parts[0] == "C" {
			continue
		}

		// Prompt line
		if parts[0] == "p" || parts[0] == "P" {
			if len(parts) < 4 {
				return nil, fmt.Errorf("invalid prompt line, expected 4 or 5 elements, got %d\n\t\t%s", len(parts), trimmedLine)
			}

			numVars, err := strconv.Atoi(parts[2])
			if err != nil {
				return nil, fmt.Errorf("could not parse numVars, expected integer, got %s", parts[2])
			}

			numClauses, err := strconv.Atoi(parts[3])
			if err != nil {
				return nil, fmt.Errorf("could not parse numClauses, expected integer, got %s", parts[3])
			}

			task = &Task{
				Name:       parts[1],
				NumVars:    numVars,
				NumClauses: numClauses,
			}
			continue
		}

		// Clause line
		clause, err := p.parseClauseLine(trimmedLine)
		if err != nil {
			return nil, fmt.Errorf("could not parse clause '%s': %v", trimmedLine, err)
		}

		clauses = append(clauses, clause)

	}

	task.Clauses = clauses

	return task, nil
}

func (p *Parser) parseClauseLine(line string) (*Clause, error) {
	parts := strings.Split(line, " ")

	clauseInts := []int{}

	// match every non-null integer, with an optional leading -
	pattern := regexp.MustCompile(`^-?[1-9]\d*$`)

	if parts[len(parts)-1] != "0" {
		return nil, fmt.Errorf("clause line does not end with a 0")
	}

	for _, part := range parts[:len(parts)-1] {
		if !pattern.MatchString(part) {
			return nil, fmt.Errorf("unexpected token %s, expected non-null integer", part)
		}

		integer, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("could not convert token %s to integer", part)
		}
		clauseInts = append(clauseInts, integer)
	}

	if !validateNoNegativePairs(clauseInts) {
		return nil, fmt.Errorf("clause contains contradicting statements")
	}

	clause := &Clause{}
	clause.Vars = make([]Variable, 0)

	for _, num := range clauseInts {

		cVar := &Variable{}

		if num < 0 {
			cVar.Negated = true
			cVar.ID = int(math.Abs(float64(num)))
		} else {
			cVar.Negated = false
			cVar.ID = num
		}

		clause.Vars = append(clause.Vars, *cVar)
	}

	return clause, nil
}

func validateNoNegativePairs(slice []int) bool {
	// Use a map to track which numbers we've seen
	seen := make(map[int]bool)

	for _, num := range slice {
		// Check if the negative of this number already exists
		if seen[-num] {
			return false
		}
		// Add current number to the set
		seen[num] = true
	}

	return true
}
