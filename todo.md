
# Core
Use Design Pattern: Strategies

- [x] Make the Default and Rave MCTS Selection,Backpropagation,setupSearch etc. (basically search.go), customizable (through the interface)
- [ ] Add more Selection policies (e.g. UCB1 with progressive bias)
  - Any selection policy is possible to implement, so this is honestly up to the user.
  - There is no clear way to implement 'biasing' without knowing the domain
- [ ] Add CLOP for optimizing MCTS parameters
  - Is this even necessary? There is no neural network and there are like, up to 2 parameters to optimize (exploration constant and rave factor)
  

### Links

- [AMAF](https://users.soe.ucsc.edu/~dph/mypubs/AMAFpaperWithRef.pdf)
- [Fast Go MCTS implementation](https://github.com/lukaszlew/libego/blob/master/source/engine/mcts_tree.cpp)
- [CLOP](https://www.remi-coulom.fr/CLOP/CLOP.pdf)