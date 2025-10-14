
# Core
Use Design Pattern: Strategies

- [x] Make the Default and Rave MCTS[T, S, R, O, A]Selection,Backpropagation,setupSearch etc. (basically search.go), customizable (through the interface)
- [x] Add more Selection policies (e.g. UCB1 with progressive bias)
  - Any selection policy is possible to implement, so this is honestly up to the user.
  - There is no clear way to implement 'biasing' without knowing the domain
- [ ] Add CLOP for optimizing MCTS[T, S, R, O, A]parameters
  - Is this even necessary? There is no neural network and there are like, up to 2 parameters to optimize (exploration constant and rave factor)
- [ ] Cool extensions for VersusArena:
  - [ ] logging of games in a file (ex: pgn for chess)
  - [ ] current game state, by default just last 5 moves played ( ex: games 15/75 | moves ... 14. A3a3 A3b2 15. B2b3 B3a1 16. A1h6 | ...)
  - [ ] engine output (ex: eval 0.45 depth 12 nodes 123456 nps 12345 time 1234 pv A3a3 A3b2 B2b3 B3a1 A1h6 ...)
  - [ ] recover on panic (save current game, current engine states etc.)
  - [ ] instead of equally distributing work between workers, use an atomic counter, so that each worker grabs a new game as soon as it finishes the previous one (some worker may finish much faster than other, especially if the number of games is huge)

## Bugs

**WaitGroup Reuse during heavy contention, in VersusArena**
```
panic: sync: WaitGroup is reused before previous Wait has returned

goroutine 13117 [running]:
sync.(*WaitGroup).Wait(0xc0001181c0?)
        /usr/local/go/src/sync/waitgroup.go:120 +0x74
github.com/IlikeChooros/go-mcts/pkg/mcts.(*MCTS[...]).Search(0x545fa0, 0xc02504bd40, 0xc015287440, 0x0)
        /home/minis/Desktop/go-mcts/pkg/mcts/search.go:172 +0x5d9
created by github.com/IlikeChooros/go-mcts/pkg/mcts.(*MCTS[...]).SearchMultiThreaded in goroutine 21
        /home/minis/Desktop/go-mcts/pkg/mcts/search.go:94 +0x185
exit status 2

```
```go
// file: pkg/mcts/search.go, function: Search(...)

// Stop every search thread
mcts.Limiter.Stop()

// Make sure only 1 thread calls this
if threadId == mainThreadId {
  mcts.invokeListener(mcts.listener.onStop)
  mcts.wg.Done()

  // Wait for other threads to finish
  mcts.wg.Wait() // <--- PANIC HAPPENS HERE
  // If we are in 'root parallel' mode, merge the results
  if mcts.shouldMerge() {
    mcts.mergeResults()
  }
} else {
  mcts.wg.Done()
}

//...

```



### Links

- [AMAF](https://users.soe.ucsc.edu/~dph/mypubs/AMAFpaperWithRef.pdf)
- [Fast Go MCTS[T, S, R, O, A]implementation](https://github.com/lukaszlew/libego/blob/master/source/engine/mcts_tree.cpp)
- [CLOP](https://www.remi-coulom.fr/CLOP/CLOP.pdf)