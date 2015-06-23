package workpool

import (
	"sync"
)

type WorkPool struct {
	size int
	wg   *sync.WaitGroup
	work chan func() // Close to terminate the workers
}

func (wp *WorkPool) run() {
	for f := range wp.work {
		f()
	}
	wp.wg.Done()
}

func New(size int, buffer int) *WorkPool {
	wp := &WorkPool{
		size: size,
		wg:   new(sync.WaitGroup),
		work: make(chan func(), buffer),
	}
	return wp
}

func (wp *WorkPool) Start() {
	for i := 0; i < wp.size; i++ {
		wp.wg.Add(1)
		go wp.run()
	}
}

func (wp *WorkPool) Close() {
	close(wp.work)
}

func (wp *WorkPool) Join() {
	wp.wg.Wait()
}

func (wp *WorkPool) Add(f func()) {
	wp.work <- f
}
