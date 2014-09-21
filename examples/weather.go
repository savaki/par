package main

import (
	"code.google.com/p/go.net/context"
	"github.com/savaki/merge"
	"github.com/savaki/openweathermap"
	"log"
	"time"
)

func ok(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func find(city string, responses chan *openweathermap.Forecast) merge.RequestFunc {
	return func(ctx context.Context) error {
		forecast, err := openweathermap.New().ByCityName(city)
		ok(err)
		responses <- forecast
		return nil
	}
}

func main() {
	// create a channel to capture our results

	forecasts := make(chan *openweathermap.Forecast, 3)

	// create our channel of requests

	requests := make(chan merge.RequestFunc, 3)
	requests <- find("San Francisco", forecasts)
	requests <- find("Oakland", forecasts)
	requests <- find("Berkeley", forecasts)
	close(requests) // important to remember to close the channel

	// execute the requests with a concurrency of 1

	resolver := merge.Requests(requests).WithRedundancy(2)
	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	err := resolver.MergeWithContext(ctx)
	cancel()
	ok(err)

	// the forecasts channel now contains all our forecasts

	close(forecasts)
	cities := map[string]*openweathermap.Forecast{}
	for forecast := range forecasts {
		cities[forecast.Name] = forecast
	}
}
