package merge_test

import (
	"bytes"
	"code.google.com/p/go.net/context"
	"github.com/savaki/merge"
	. "github.com/visionmedia/go-debug"
	"io"
	"net/http"
	"testing"
	"time"
)

var debug = Debug("merge_test")

type weather struct {
	city  string
	value string
}

func FindWeather(city string, results chan weather) merge.RequestFunc {
	_city := city
	return func(ctx context.Context) error {
		request, _ := http.NewRequest("GET", "http://api.openweathermap.org/data/2.5/weather?q="+_city, nil)
		return merge.Do(ctx, request, func(response *http.Response, err error) error {
			if err != nil {
				return err
			}
			defer response.Body.Close()

			// extract the body of the response and toss it onto the results channel
			buffer := bytes.NewBuffer([]byte{})
			io.Copy(buffer, response.Body)
			results <- weather{_city, buffer.String()}

			return nil
		})
	}
}

func TestMerger(t *testing.T) {
	// Given a channel of requests
	redundancy := 2
	cities := []string{
		"San Francisco",
		"Oakland",
		"Berkeley",
		"Palo Alto",
		"San Jose",
	}

	requests := make(chan merge.RequestFunc, len(cities))
	results := make(chan weather, len(cities)*redundancy) // buffer for clarity of example
	for _, city := range cities {
		requests <- FindWeather(city, results)
	}
	close(requests)

	// When
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	merger := merge.
		Requests(requests).
		WithRedundancy(redundancy).
		WithConcurrency(3)
	err := merger.MergeWithContext(ctx)

	// Then - I expect success
	if err != nil {
		t.Fail()
	}

	// And - I can easily extract my resulting values
	close(results)
	allWeathers := map[string]string{}
	for result := range results {
		allWeathers[result.city] = result.value
	}
}
