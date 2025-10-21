# go-mcts

An implementation of generic [Monte-Carlo Tree Search](https://en.wikipedia.org/wiki/Monte_Carlo_tree_search) in Go, with high configuration support.

## Features
- **Selection policies**: UCB1 and RAVE (AMAF)
- **Multithreading modes**:
  - Root-parallel: independent per-thread roots, merged at the end
  - Tree-parallel: shared synchronized tree with atomic operations
- **Live statistics**: depth, tree size, cycles per second, principal variation via listener callbacks
- **Flexible limits**: time, memory, depth, and cycle count
- **Custom backpropagation**: supports 2+ player games via strategy pattern
- **Generic API**: parameterized over move type, node stats, and game result
- **Versus arena**: benchmarking tool for head-to-head engine comparisons across multiple threads
- **Real-world examples**:
  - Ultimate Tic-Tac-Toe with UCB1 and RAVE
  - Chess with UCB1 and RAVE (using dragontoothmg for rules/move generation)

## Requirements
- Go 1.22+

## Installation
```sh
go get github.com/IlikeChooros/go-mcts
```

This package uses [`termenv`](https://github.com/muesli/termenv) for terminal styling in the benchmarking subpackage.

## Quick start

```go
package main

import (
    "fmt"
    mcts "github.com/IlikeChooros/go-mcts/pkg/mcts"
)

// Define your Move type and implement GameOperations.
// For brevity, this is a sketch; see examples for full code.

type Move int

type Ops struct{ /* game state */ }

func (o *Ops) ExpandNode(p *mcts.NodeBase[Move, *mcts.NodeStats]) uint32 { /* ... */ return 0 }
func (o *Ops) Traverse(m Move) { /* ... */ }
func (o *Ops) BackTraverse() { /* ... */ }
func (o *Ops) Rollout() mcts.Result { /* ... */ return 0.5 }
func (o *Ops) Reset() {}
func (o *Ops) Clone() *Ops {
    return &Ops{/* deep copy */}
}

func main() {
    ops := &Ops{}
    
    // Create a selection policy (UCB1 with exploration constant 0.4)
    policy := mcts.NewUCB1[Move, *mcts.NodeStats, mcts.Result, *Ops](0.4)
    
    // Build the MCTS tree
    tree := mcts.NewMTCS(
        policy,
        ops,
        mcts.MultithreadTreeParallel,
        &mcts.NodeStats{},
    )

    tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(4))
    tree.Search()

    best := tree.BestMove()
    fmt.Println("Best move:", best)
}
```

## RAVE Usage
- Use [`mcts.NewRAVE`](pkg/mcts/rave.go) as the selection policy
- Node stats must implement [`RaveStatsLike`](pkg/mcts/rave.go) (use the provided [`RaveStats`](pkg/mcts/rave.go))
- Rollout must return a [`RaveGameResult`](pkg/mcts/rave.go) containing:
  - The playout outcome in [0,1]
  - Moves played by each side (for AMAF updates)
- Backpropagation strategy is automatically handled by RAVE (updates AMAF counters during tree ascent)

See complete examples:
- [Ultimate Tic-Tac-Toe with RAVE](examples/ultimate-tic-tac-toe/rave/main.go)
- [Chess with RAVE](examples/chess/rave/main.go)

## Examples

**Ultimate Tic Tac Toe**
- UCB1: [/examples/ultimate-tic-tac-toe/main.go](./examples/ultimate-tic-tac-toe/main.go)
- RAVE + real-time stats: [/examples/ultimate-tic-tac-toe/rave/main.go](./examples/ultimate-tic-tac-toe/rave/main.go)

Run an example:
```sh
go run ./examples/ultimate-tic-tac-toe/main.go
# or
go run ./examples/ultimate-tic-tac-toe/rave/main.go
```

**Chess**
- UCB1: [/examples/chess/main.go](./examples/chess/main.go)
- RAVE + real-time stats: [/examples/chess/rave/main.go](./examples/chess/rave/main.go)

>[!NOTE]
> Chess examples require dragontoothmg for move generation:
> ```sh
> cd examples/chess
> go get github.com/IlikeChooros/dragontoothmg
> ```

Run an example:
```sh
go run ./examples/chess/main.go
# or
go run ./examples/chess/rave/main.go
```

**Benchmarking with Versus Arena**

The [`pkg/bench`](pkg/bench) subpackage provides a head-to-head arena for comparing MCTS configurations:

```go
arena := bench.NewVersusArena(startPosition, player1MCTS, player2MCTS)
arena.Setup(limits, totalGames, workerThreads)
arena.Start("UCB1", "RAVE", listener)
arena.Wait()

results := arena.Results()
fmt.Printf("P1 wins: %d, P2 wins: %d, Draws: %d\n", 
    results.P1Wins, results.P2Wins, results.Draws)
```

See [examples/ultimate-tic-tac-toe/bench](examples/ultimate-tic-tac-toe/bench) for a full implementation.

## Concurrency and Performance
- **Tree-parallel** uses atomic operations; [`CollisionFactor()`](pkg/mcts/mcts.go) indicates contention on node expansions
- **Root-parallel** scales better for high thread counts but delays listener updates until merge
- Listeners can impact search speed if called too frequently or perform heavy operations; use [`SetCycleInterval`](pkg/mcts/stats_listener.go) to throttle

## Docs

https://pkg.go.dev/github.com/IlikeChooros/go-mcts

Quick overview:

**Core types** (generic):
- [`MCTS[T, S, R, O, A]`](pkg/mcts/mcts.go): main tree search controller
- [`Strategy[T, S, R, O]`](pkg/mcts/strategy.go): interface for backpropagation and selection strategies
- [`GameOperations[T, S, R, O]`](pkg/mcts/ops.go): interface for game rules
- [`NodeBase[T, S]`](pkg/mcts/node.go), [`NodeStats`](pkg/mcts/node.go), [`RaveStats`](pkg/mcts/rave.go): tree node and statistics
- [`VersusArena[...]`](pkg/bench/versus_arena.go): benchmarking harness

**Key methods**:
- Tree control: [`SetLimits`](pkg/mcts/limits.go), [`Search`](pkg/mcts/mcts.go), [`Stop`](pkg/mcts/mcts.go), [`IsSearching`](pkg/mcts/mcts.go)
- Move selection: [`BestMove`](pkg/mcts/mcts.go), [`BestChild`](pkg/mcts/mcts.go), [`Pv`](pkg/mcts/mcts.go), [`MultiPv`](pkg/mcts/mcts.go)
- Statistics: [`MaxDepth`](pkg/mcts/mcts.go), [`Cycles`](pkg/mcts/mcts.go), [`Cps`](pkg/mcts/mcts.go), [`CollisionFactor`](pkg/mcts/mcts.go), [`Size`](pkg/mcts/mcts.go), [`MemoryUsage`](pkg/mcts/mcts.go)

## License
MIT