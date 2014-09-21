package merge

import (
	"code.google.com/p/go.net/context"
	"errors"
	"fmt"
	. "github.com/visionmedia/go-debug"
)

var debug = Debug("merge")

type RequestFunc func(ctx context.Context) error

type Merger interface {
	WithRedundancy(redundancy int) Merger
	WithConcurrency(concurrency int) Merger
	Merge() error
	MergeWithContext(ctx context.Context) error
}

type merger struct {
	requests    chan RequestFunc
	redundancy  int
	concurrency int
}

func Requests(requests chan RequestFunc) Merger {
	return &merger{
		requests:    requests,
		redundancy:  1,
		concurrency: 0,
	}
}

func (m *merger) WithRedundancy(redundancy int) Merger {
	return &merger{
		requests:    m.requests,
		redundancy:  redundancy,
		concurrency: m.concurrency,
	}
}

func (m *merger) WithConcurrency(concurrency int) Merger {
	return &merger{
		requests:    m.requests,
		redundancy:  m.redundancy,
		concurrency: concurrency,
	}
}

type response struct {
	id  int
	err error
}

func (m merger) enqueue(ctx context.Context, responses chan *response, done chan interface{}) int {
	// helper method to execute request and toss response into responses channel
	handle := func(id int, request RequestFunc, pool <-chan interface{}) {
		defer func() { <-pool }()
		i := id
		debug(fmt.Sprintf("request: %d", i))
		err := request(ctx)
		responses <- &response{
			id:  i,
			err: err,
		}
	}

	// materialize the requests channel into an array of requests
	var requests []RequestFunc
	for request := range m.requests {
		requests = append(requests, request)
	}

	// use a go routine execute calls to our remote service as per the specified
	// level of concurrency
	go func() {
		// create a channel to simulate both a bounded and unbounded pool
		pool := makePoolChan(m.concurrency)
		defer close(pool)

		for attempt := 1; attempt <= m.redundancy; attempt++ {
			for id, request := range requests {
				select {
				case pool <- true:
					go handle(id, request, (<-chan interface{})(pool))
				case <-done:
					return
				}
			}
		}
	}()

	return len(requests)
}

func (m merger) Merge() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return m.MergeWithContext(ctx)
}

func (m merger) MergeWithContext(ctx context.Context) error {
	// internal communication channel
	responses := make(chan *response)

	// signal to indicate we're done
	done := make(chan interface{})
	defer close(done)

	// create a marker channel to let us know once we have pushed all the requests
	// onto the queue
	expected := m.enqueue(ctx, responses, done)

	// collect the results and return when finished
	results := map[int]int{}
	for expected != len(results) {
		select {
		case response := <-responses:
			if response.err == nil {
				results[response.id] = response.id
				debug(fmt.Sprintf("received - %d", response.id))
			}
		case <-ctx.Done():
			debug("timeout")
			return errors.New("must have timed out")
		}
	}

	debug("finished")
	return nil
}
