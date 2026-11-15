package util

import "sync"

// ForEach calls fn(i) for every i in [0, n) on at most workers goroutines and
// returns when all calls have finished. Each call should write only to its own
// slot of any shared slice, which makes the results safe to read afterwards.
func ForEach(n, workers int, fn func(i int)) {
	if n <= 0 {
		return
	}
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}

	indexes := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		indexes <- i
	}
	close(indexes)
	wg.Wait()
}
