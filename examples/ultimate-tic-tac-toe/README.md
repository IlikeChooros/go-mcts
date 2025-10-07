## Ultimate Tic Tac Toe Example

These are examples of ultimate tic tac toe implementation.
If you don't know the rules, see: https://en.wikipedia.org/wiki/Ultimate_tic-tac-toe


The [`main.go`](./main.go) file has basic instructions on how to use the mcts package, with implemented interface in [`uttt/ucb/uttt_mcts.go`](./uttt/ucb/uttt_mcts.go).
On how to use RAVE as selection policy, see [`uttt/rave/uttt_mcts.go`](./uttt/rave/uttt_mcts.go)

For more advanced usage with real-time search stats, see [reat-time-stats/main.go](./real-time-stats/main.go), it showcases how to use the [`Listener`](../../pkg/mcts/stats_listener.go), with `OnStop`, `OnDepth` and `OnCycle` methods.
