package merge

func makePoolChan(concurrent int) chan interface{} {
	var ch chan interface{}

	if concurrent == 0 {
		ch = make(chan interface{}, 16384)
		go func() {
			for _ = range ch {
				// we make an unbounded pool by creating a bounded pool that drains very fast
			}
		}()

	} else {
		ch = make(chan interface{}, concurrent)
	}

	return ch
}
