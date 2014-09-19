package merge

import (
	"code.google.com/p/go.net/context"
	"errors"
	"fmt"
	. "github.com/visionmedia/go-debug"
	"time"
)

var debug = Debug("merge")

type RequestFunc func(ctx context.Context) error

type Merger interface {
	WithParallelism(parallelism int) Merger
	Merge() error
}

type merger struct {
	requests    chan RequestFunc
	timeout     time.Duration
	parallelism int
}

func Requests(requests chan RequestFunc, timeout time.Duration) Merger {
	return &merger{
		requests:    requests,
		timeout:     timeout,
		parallelism: 1,
	}
}

func (m merger) WithParallelism(parallelism int) Merger {
	return &merger{
		requests:    m.requests,
		timeout:     m.timeout,
		parallelism: parallelism,
	}
}

type response struct {
	id  int
	err error
}

func (m merger) Merge() error {
	// cancel all things that we're done with
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	// internal communication channel
	responses := make(chan response)

	// helper method to execute request and toss response into responses channel
	handle := func(id int, request RequestFunc) {
		i := id
		debug(fmt.Sprintf("request: %d", i))
		err := request(ctx)
		responses <- response{
			id:  i,
			err: err,
		}
	}

	// perform each request with two parallel calls
	id := 0
	for request := range m.requests {
		id = id + 1
		for agent := 0; agent < m.parallelism; agent++ {
			go handle(id, request)
		}
	}

	// collect the results and return when finished
	results := map[int]int{}
	for len(results) < id {
		select {
		case result := <-responses:
			if result.err == nil {
				results[result.id] = result.id
				debug(fmt.Sprintf("received - %d", result.id))
			}
		case <-ctx.Done():
			debug("timeout")
			return errors.New("must have timed out")
		}
	}

	debug("finished")
	return nil
}
