# go-mcts

An implementation of generic [Monte-Carlo Tree Search](https://wikipedia.com) in Go, with high configuration support.

## Features
- Support 2 mulithreading policies: tree parallel (each search thread has it's own copy of the root tree, then mergin results) and root parallel (all threads work on the same synchronized tree)
- Has listeners for the tree statistics (depth, cycles per second, principal variation and other)

## Usage

This algorithm should be used only with zero-sum, turn based games, where implementing traditional evalutaion function wouldn't be trivial.
See [examples](./examples/) for implemntation schemes.

To download the package use:
```
go get github.com/IlikeChooros/go-mcts
```


## License
MIT