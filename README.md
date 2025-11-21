# DPLL SAT Solver

A high-performance SAT solver implementing the DPLL (Davis-Putnam-Logemann-Loveland) algorithm with support for both sequential and parallel solving modes.

## Features

- **Sequential Solver**: Classic DPLL algorithm with unit propagation, pure literal elimination, and backtracking
- **Parallel Solver**: Multi-threaded work-stealing implementation for improved performance
- **Optimum Mode**: Exhaustively searches for the minimal solution (fewest variable assignments)
- **Configurable Logging**: Multiple log levels for debugging and analysis
- **DIMACS Format**: Standard CNF input format support

## Building

```bash
go build -o dpll-solver
```

## Usage

```bash
./dpll-solver [options] <input-file>
```

### Command-Line Options

| Flag               | Short | Description                                                                     | Default                |
| ------------------ | ----- | ------------------------------------------------------------------------------- | ---------------------- |
| `--log-level`      | `-l`  | Log level: `none`, `steps`, or `full`                                           | `none`                 |
| `--parallel`       | `-p`  | Enable parallel solving                                                         | `false`                |
| `--threads`        | `-t`  | Number of worker threads (requires `--parallel`)                                | Half of available CPUs |
| `--parallel-depth` | `-d`  | Only parallelize splits up to this depth (0 = unlimited, requires `--parallel`) | `0`                    |
| `--optimum`        | `-o`  | Find minimal solution with fewest variable assignments (requires `--parallel`)  | `false`                |

## Examples

### Sequential Solving (Default)

```bash
# Basic sequential solve
./dpll-solver problem.cnf

# Sequential with detailed logging
./dpll-solver problem.cnf --log-level steps
```

### Parallel Solving

```bash
# Parallel solve with 4 workers
./dpll-solver problem.cnf --parallel --threads 4

# Parallel solve with default thread count (half of CPU cores)
./dpll-solver problem.cnf --parallel

# Parallel solve with depth limit (only parallelize first 3 levels)
./dpll-solver problem.cnf --parallel --threads 8 --parallel-depth 3
```

### Optimum Mode

```bash
# Find minimal solution (exhaustive search)
./dpll-solver problem.cnf --parallel --threads 4 --optimum

# Find minimal solution with logging
./dpll-solver problem.cnf --parallel --threads 4 --optimum --log-level steps
```

**Note:** Optimum mode will print each improved solution as it's found, then report the final optimal solution.

## Input Format

The solver accepts CNF files in DIMACS format:

```
c Comments start with 'c'
p cnf 4 3
1 2 -3 0
-1 -2 3 0
1 -2 0
```

- `p cnf <variables> <clauses>`: Problem line defining number of variables and clauses
- Each clause is a space-separated list of literals (negative = negated) ending with `0`
- Variable IDs are positive integers starting from 1

## Algorithm Details

### Sequential Solver

The sequential solver implements the classic DPLL algorithm:

1. **Unit Propagation**: Automatically assigns variables appearing alone in clauses
2. **Pure Literal Elimination**: Assigns variables that appear with only one polarity
3. **Splitting**: Chooses the most frequently occurring variable and explores both assignments
4. **Backtracking**: Returns to previous decision points when contradictions are found

### Parallel Solver

The parallel solver extends DPLL with work-stealing parallelism:

- **Work Queue**: Shared queue of unexplored search branches
- **Worker Threads**: Multiple workers process branches concurrently
- **Dynamic Load Balancing**: Workers steal work from the queue when idle
- **Memory Management**: Queue size limits prevent exponential memory growth
- **Early Termination**: All workers stop once a solution is found (normal mode)

### Optimum Mode

Optimum mode exhaustively explores the search space to find the solution with the fewest variable assignments:

- Workers continue after finding solutions to explore alternative branches
- Each new solution is compared against the current best
- Better solutions are reported as they're discovered
- Search terminates only when all branches are exhausted
- Guarantees finding the minimal solution

## Performance Considerations

### Thread Count

- Default: Half of available CPU cores (good starting point)
- For small problems: 2-4 threads may be optimal
- For large problems: Scale up to match available cores
- Diminishing returns beyond 8-16 threads for most problems

### Parallel Depth

- Default (0): Unlimited parallelization
- Setting a depth limit (e.g., 3-5) can reduce memory usage
- Lower depths mean less parallelism but more manageable memory footprint
- Useful for very large problems where memory is constrained

### Optimum Mode

- **Warning**: Optimum mode is much slower than normal solving
- Requires exploring the entire search space
- Memory usage is managed through sequential backtracking when queue is full
- Best suited for problems where solution quality matters more than speed

## Logging Levels

### `none` (default)

Only shows the final result and solution.

### `steps`

Shows major steps: unit propagation, pure literal elimination, splits, and backtracking.

### `full`

Shows detailed debug information including worker activity in parallel mode.

## Exit Codes

- `0`: Success (solution found or proven unsatisfiable)
- `1`: Error (file not found, parsing error, or invalid input)

## Examples with Output

### Sequential Solve

```bash
$ ./dpll-solver uf20-91/uf20-01.cnf
Starting to solve 91 clauses.
Found solution: { 15 -5 -12 -7 19 10 11 17 4 14 2 -13 9 -1 -6 8 3 -16 20 18 }
Result: SATISFIABLE
Solution: { 15 -5 -12 -7 19 10 11 17 4 14 2 -13 9 -1 -6 8 3 -16 20 18 }
```

### Parallel Solve

```bash
$ ./dpll-solver uf20-91/uf20-01.cnf --parallel --threads 4
Using 4 worker threads
Starting parallel solver with 4 workers
Worker 1: Found solution: { 15 -5 -12 -7 19 10 17 20 14 11 -13 4 9 -6 -1 8 3 -16 2 18 }
Result: SATISFIABLE
Solution: { 15 -5 -12 -7 19 10 17 20 14 11 -13 4 9 -6 -1 8 3 -16 2 18 }
```

### Optimum Mode

```bash
$ ./dpll-solver uf20-91/uf20-01.cnf --parallel --threads 4 --optimum
Using 4 worker threads
Starting parallel solver with 4 workers
Found solution with 20 variables: { 15 -5 -12 -7 19 10 11 17 20 14 -13 4 9 -6 -1 8 3 2 -16 18 }
Found solution with 19 variables: { 15 -5 -12 -7 -19 20 14 -2 -11 -8 17 -16 1 -18 -3 6 13 -10 -9 }

Optimal solution found with 19 variables: { 15 -5 -12 -7 -19 20 14 -2 -11 -8 17 -16 1 -18 -3 6 13 -10 -9 }
Result: SATISFIABLE
Solution: { 15 -5 -12 -7 -19 20 14 -2 -11 -8 17 -16 1 -18 -3 6 13 -10 -9 }
```

## Implementation Notes

- Written in Go for performance and concurrency
- Uses condition variables for efficient worker synchronization (no busy-waiting)
- Implements proper termination detection in parallel mode
- Queue size limits prevent memory exhaustion
- Deep copying of work items ensures thread safety

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Authors

CptPie
