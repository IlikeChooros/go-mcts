package bench

import "github.com/IlikeChooros/go-mcts/pkg/mcts"

// TODO:
// Will distribute the proper stats between the VersusArena and Listeners

type ArenaListener[T mcts.MoveLike] struct {
	listeners   []ListenerLike[T]
	workerInfos []chan VersusWorkerInfo[T]
}

func NewArenaListener[T mcts.MoveLike](nWorkers int) *ArenaListener[T] {
	al := &ArenaListener[T]{
		listeners: make([]ListenerLike[T], 0, nWorkers),
	}
	for i := 0; i < nWorkers; i++ {
		al.listeners = append(al.listeners, &DefaultListener[T]{row: i})
	}
	return al
}
