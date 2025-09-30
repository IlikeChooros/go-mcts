# go-mcts

An implementation of generic [Monte-Carlo Tree Search](https://en.wikipedia.org/wiki/Monte_Carlo_tree_search) in Go, with high configuration support.

## Features
- RAVE and UCB1 selection policies
- Two multihtreading modes:
  - Root-parallel: independent per-thread roots, merged at the end of the search
  - Tree-parallel: shared synchronized tree
- Live stats via listener, including depth, size, cycles per second, PV
- Vast limiting support, including: time, memory and depth
- Allows custom backpropagation strategy (supports 2+ player games)
- Generic API over move type and stats
- Useful, not typical examples:
  - Ultimate Tic-Tac-Toe with UCB1 and RAVE
  - Chess with UCB1 and RAVE (using dragontoothmg engine for rules, move gen, make/undo)

## Requirements
- Go 1.22+

## Instalation
```
go get github.com/IlikeChooros/go-mcts
```

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
func (o *Ops) Clone() mcts.GameOperations[Move, *mcts.NodeStats, mcts.Result] {
    return &Ops{/* deep copy */}
}

func main() {
    ops := &Ops{}
    tree := mcts.NewMTCS[Move, *mcts.NodeStats, mcts.Result](
        mcts.UCB1,
        ops,
        0, // flags; use mcts.TerminalFlag if root is terminal
        mcts.MultithreadTreeParallel,
        &mcts.NodeStats{},
        mcts.DefaultBackprop[Move, *mcts.NodeStats, mcts.Result]{},
    )

    tree.SetLimits(mcts.DefaultLimits().SetMovetime(2000).SetThreads(4))
    tree.Search()

    best := tree.RootMove()
    fmt.Println("Best move:", best)
}
```

## RAVE Usage
- Use RAVE as selection policy and node stats that implemenet RaveStatsLike (use the default: RaveNodeStats)
- Rollout must return moves that were played and a float value of the result
- Backpropagation must update AMAF counters with moves seen during rollout (default: RaveBackprop)

See a complete example:
- Ultimate Tic-Tac-Toe with RAVE and live stats: examples/ultimate-tic-tac-toe/uttt/rave

## Examples

**Ultimate Tic Tac Toe**
- UCB1: [/examples/ultimate-tic-tac-toe/main.go](./examples/ultimate-tic-tac-toe/main.go)
- RAVE + real-time stats: [/examples/ultimate-tic-tac-toe/rave/main.go](./examples/ultimate-tic-tac-toe/rave/main.go)

Run an example
```sh
go run ./examples/ultimate-tic-tac-toe/main.go
# or
go run ./examples/ultimate-tic-tac-toe/rave/main.go
```

**Chess**
- UCB1: [/examples/chess/main.go](./examples/chess/main.go)
- RAVE + real-time stats: [/examples/chess/rave/main.go](./examples/chess/rave/main.go)

Run an example

>[!NOTE]
> Because chess move generation and rules are not implemented here, you need to get dragontoothmg:
> ```sh
> cd examples/chess
> go get github.com/IlikeChooros/dragontoothmg
> ```

```sh
go run ./main.go
# or
go run ./rave/main.go
```

## Concurrency and Performance
- Tree-parallel uses atomics. CollisionFactor indicates waiting on expansions.
- Listeners can slow searches if called too frequently or execute costly functions.
- Root-parallel improves scaling at the cost of delayed listener accuracy until merge.

## Docs

https://pkg.go.dev/github.com/IlikeChooros/go-mcts

Quick overview:

Core types (generic):
- MCTS[T MoveLike, S NodeStatsLike, R GameResult]
- SelectionPolicy[T, S]
- GameOperations[T, S, R] and RandGameOperations[T, S, R]
- Strategies for backpropagation (DefaultBackprop, RaveBackprop)
- NodeBase[T, S], NodeStats and RaveStats

Key methods:
- SetLimits, Search, Stop, IsSearching
- RootMove, RootScore, BestChild, Pv, MultiPv
- Stats: MaxDepth, Cycles, Cps, CollisionFactor

## License
MIT