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
func toListenerStats[T MoveLike, S NodeStatsLike](tree *MCTS[T, S]) ListenerTreeStats[T] {
	pv := tree.MultiPv(BestChildMostVisits)
	lines := make([]SearchLine[T], len(pv))
	for i := range len(pv) {
		lines[i] = SearchLine[T]{
			BestMove: pv[i].Root.NodeSignature,
			Moves:    pv[i].Pv,
			Eval:     float64(pv[i].Root.Stats.AvgOutcome()),
			Terminal: pv[i].Terminal,
			Draw:     pv[i].Draw,
		}
	}

	return ListenerTreeStats[T]{
		Lines:      lines,
		Maxdepth:   int(tree.MaxDepth()),
		Cycles:     int(tree.Root.Stats.Visits()),
		TimeMs:     int(tree.Limiter.Elapsed()),
		Cps:        tree.Cps(),
		Size:       tree.Size(),
		StopReason: tree.Limiter.StopReason(),
	}
}

// Listener function callback, will recieve current tree statistics, like
// max depth of tree, number of iterations so far
type ListenerFunc[T MoveLike] func(ListenerTreeStats[T])

type StatsListener[T MoveLike, S NodeStatsLike] struct {
	// called when 'max depth' increases, receives new max depth
	onDepth ListenerFunc[T]

	// called every N full iterations, receives total number of cycles
	onCycle ListenerFunc[T]
	nCycles int // call 'onCycle' every N cycles

	// called when the search stops (either by limiter or 'stop' signal)
	onStop ListenerFunc[T]
}

func NewStatsListener[T MoveLike, S NodeStatsLike]() StatsListener[T, S] {
	return StatsListener[T, S]{nCycles: 1}
}

// Attach new on max depth change callback, will be called only be the main search thread,
// meaning no need for synchronization here
func (listener *StatsListener[T, S]) OnDepth(onDepth ListenerFunc[T]) *StatsListener[T, S] {
	listener.onDepth = onDepth
	return listener
}

// Attach new on iteration increase callback, this will significantly slow down the search,
// because of pv evaluation, so use it only for debugging
func (listener *StatsListener[T, S]) OnCycle(onCycle ListenerFunc[T]) *StatsListener[T, S] {
	listener.onCycle = onCycle
	return listener
}

func (listener *StatsListener[T, S]) invokeCycle(tree *MCTS[T, S]) {
	if listener.onCycle != nil && tree.Root.Stats.Visits()%int32(listener.nCycles) == 0 {
		listener.onCycle(toListenerStats(tree))
	}
}

func (listener *StatsListener[T, S]) SetCycleInterval(n int) *StatsListener[T, S] {
	if n < 1 {
		n = 1
	}
	listener.nCycles = n
	return listener
}

// Attach 'on search end' callback, called once by the main thread,
// makes 'StopReason' available in the stats
func (listener *StatsListener[T, S]) OnStop(onStop ListenerFunc[T]) *StatsListener[T, S] {
	listener.onStop = onStop
	return listener
}
