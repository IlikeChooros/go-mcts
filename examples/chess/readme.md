## Chess Example

These are examples of integrating go-mcts with a chess engine (dragontoothmg).

All chess rules, move generation, and make/undo are provided by the third‑party engine:
- [github.com/IlikeChooros/dragontoothmg](https://github.com/IlikeChooros/dragontoothmg)

The [`main.go`](./main.go) file shows a basic MCTS setup using UCB1, with the chess-specific
GameOperations implemented in [`chess-mcts/ucb.go`](./chess-mcts/ucb.go).

For a more advanced implementation with RAVE (AMAF) as the selection policy, see
[`rave/main.go`](./rave/main.go) and the chess integration in [`chess-mcts/rave.go`](./chess-mcts/rave.go).
It also demonstrates how to customize the RAVE beta function.

Both examples print real-time search info (depth, eval, PV) using the MCTS listener:
see [`Listener`](../../pkg/mcts/stats_listener.go) with `OnStop`, `OnDepth`, and `OnCycle` hooks.