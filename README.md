# go-mcts

An implementation of generic [Monte-Carlo Tree Search](https://en.wikipedia.org/wiki/Monte_Carlo_tree_search) in Go, with high configuration support.

## Features
- Support 2 mulithreading policies: root parallel (each search thread has it's own copy of the root tree, then merging results) and tree parallel (all threads work on the same synchronized tree)
- Has listeners for the tree statistics (depth, cycles per second, principal variation and other)
- Vast limiting support, including: time, memory and depth
- 
*Incoming*
- add RAVE as selection policy
- add leaf paralellization
- cancel with context


## Usage

This algorithm should be used only with zero-sum, turn based games, where implementing traditional evalutaion function wouldn't be trivial.
See [examples](./examples/) for implemntation schemes.

To download the package use:
```
go get github.com/IlikeChooros/go-mcts
```


## License
MIT