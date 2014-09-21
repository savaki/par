package par

import (
	"code.google.com/p/go.net/context"
	"net/http"
)

type ResponseHandlerFunc func(*http.Response, error) error

func Do(ctx context.Context, req *http.Request, f ResponseHandlerFunc) error {
	// Run the HTTP request in a go routine and pass the response to f.
	tr := &http.Transport{}
	client := &http.Client{Transport: tr}
	c := make(chan error, 1)

	// execute our request in a goroutine
	go func() { c <- f(client.Do(req)) }()

	// wait for either the response or a cancel message
	select {
	case <-ctx.Done():
		debug("http - CancelRequest")
		tr.CancelRequest(req)
		<-c // Wait for f to return.
		return ctx.Err()
	case err := <-c:
		debug("http - ok")
		return err
	}
}
