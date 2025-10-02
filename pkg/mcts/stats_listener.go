package mcts

type SearchLine[T MoveLike] struct {
	BestMove T
	Moves    []T
	Eval     float64
	Terminal bool
	Draw     bool
}

type ListenerTreeStats[T MoveLike] struct {
	Maxdepth   int
	Cycles     int
	TimeMs     int
	Cps        uint32
	Size       uint32
	Lines      []SearchLine[T]
	StopReason StopReason
}

// Convert TreeStats to 'ListenerTreeStats' struct
func toListenerStats[T MoveLike, S NodeStatsLike, R GameResult](tree *MCTS[T, S, R]) ListenerTreeStats[T] {
	pv := tree.MultiPv(BestChildMostVisits)
	lines := make([]SearchLine[T], len(pv))
	for i := range len(pv) {
		lines[i] = SearchLine[T]{
			BestMove: pv[i].Root.Move,
			Moves:    pv[i].Pv,
			Eval:     float64(pv[i].Root.Stats.AvgQ()),
			Terminal: pv[i].Terminal,
			Draw:     pv[i].Draw,
		}
	}

	return ListenerTreeStats[T]{
		Lines:      lines,
		Maxdepth:   int(tree.MaxDepth()),
		Cycles:     int(tree.Root.Stats.N()),
		TimeMs:     int(tree.Limiter.Elapsed()),
		Cps:        tree.Cps(),
		Size:       tree.Size(),
		StopReason: tree.Limiter.StopReason(),
	}
}

// Listener function callback, will recieve current tree statistics, like
// max depth of tree, number of iterations so far
type ListenerFunc[T MoveLike] func(ListenerTreeStats[T])

type StatsListener[T MoveLike] struct {
	// called when 'max depth' increases, receives new max depth
	onDepth ListenerFunc[T]

	// called every N full iterations, receives total number of cycles
	onCycle ListenerFunc[T]
	nCycles int // call 'onCycle' every N cycles

	// called when the search stops (either by limiter or 'stop' signal)
	onStop ListenerFunc[T]
}

func NewStatsListener[T MoveLike]() StatsListener[T] {
	return StatsListener[T]{nCycles: 1}
}

// Attach new on max depth change callback, will be called only be the main search thread,
// meaning no need for synchronization here
func (listener *StatsListener[T]) OnDepth(onDepth ListenerFunc[T]) *StatsListener[T] {
	listener.onDepth = onDepth
	return listener
}

// Attach new on iteration increase callback, this will significantly slow down the search,
// because of pv evaluation, so use it only for debugging
func (listener *StatsListener[T]) OnCycle(onCycle ListenerFunc[T]) *StatsListener[T] {
	listener.onCycle = onCycle
	return listener
}

func (listener *StatsListener[T]) SetCycleInterval(n int) *StatsListener[T] {
	listener.nCycles = max(1, n)
	return listener
}

// Attach 'on search end' callback, called once by the main thread,
// makes 'StopReason' available in the stats
func (listener *StatsListener[T]) OnStop(onStop ListenerFunc[T]) *StatsListener[T] {
	listener.onStop = onStop
	return listener
}
