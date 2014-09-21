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
	Merge() error
}

type merger struct {
	requests   chan RequestFunc
	ctx        context.Context
	redundancy int
}

func Requests(requests chan RequestFunc) Merger {
	return &merger{
		requests:   requests,
		ctx:        context.Background(),
		redundancy: 1,
	}
}

func (m merger) WithRedundancy(redundancy int) Merger {
	return &merger{
		requests:   m.requests,
		ctx:        m.ctx,
		redundancy: redundancy,
	}
}

func (m merger) WithContext(ctx context.Context) Merger {
	return &merger{
		requests:   m.requests,
		ctx:        ctx,
		redundancy: m.redundancy,
	}
}

type response struct {
	id  int
	err error
}

func (m merger) Merge() error {
	// internal communication channel
	responses := make(chan response)

	// helper method to execute request and toss response into responses channel
	handle := func(id int, request RequestFunc) {
		i := id
		debug(fmt.Sprintf("request: %d", i))
		err := request(m.ctx)
		responses <- response{
			id:  i,
			err: err,
		}
	}

	// perform each request with two redundant calls
	id := 0
	for request := range m.requests {
		id = id + 1
		for agent := 0; agent < m.redundancy; agent++ {
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
		case <-m.ctx.Done():
			debug("timeout")
			return errors.New("must have timed out")
		}
	}

	debug("finished")
	return nil
}
