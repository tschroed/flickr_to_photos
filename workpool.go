package main

import (
	"fmt"
	"sync"
	"time"
)

type WorkPool struct {
	size int
	wg *sync.WaitGroup
	work chan func()  // Close to terminate the workers
}

func (wp *WorkPool) run() {
	fmt.Println("Entering loop")
	for f := range wp.work {
		f()
	}
	fmt.Println("And done")
	wp.wg.Done()
}

func New(size int, buffer int) *WorkPool {
	wp := &WorkPool{
		size: size,
		wg: new(sync.WaitGroup),
		work: make(chan func(), buffer),
	}
	return wp
}

func (wp *WorkPool) Start() {
	for i := 0 ; i < wp.size ; i++ {
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

func main() {
	f := func() {
		fmt.Printf("Sleeping...")
		time.Sleep(5 * time.Second)
		fmt.Println("Done.")
	}
	wp := New(10, 0)
	wp.Start()
	for i := 0; i < 20; i++ {
		wp.Add(f)
	}
	fmt.Println("Closing")
	wp.Close()
	fmt.Println("Joining")
	wp.Join()
	fmt.Println("Exiting")
}
