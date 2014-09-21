merge
=====

merge is a Go library to process calls with the following characteristics:

* calls will be processed in parallel 
* parallelism may be optionally bounded
* leverages Google's Context library to allow calls to be timed our or actively canceled

## Example - Find the weather in 3 cities with a concurrency of 1

```
import (
	"github.com/savaki/merge"
	"github.com/savaki/openweathermap"
	"log"
)

fund find(city string, responses chan *Weather) merge.RequestFunc {
	return func(ctx context.Context) error {
		forecast, err := openweathermap.New().FindByCity(city)
		if err != nil {
			return err		}
		responses <- forecast
		return nil	}
}

func main() {
	// create a channel to capture our results
	results := make(chan openweathermap.Weather, 3)
	
	// create our channel of requests
	requests := make(chan merge.RequestFunc, 3)
	requests <- find("San Francisco")
	requests <- find("Oakland")
	requests <- find("Berkeley")
	close(requests) // important to remember to close the channel
	
	// execute the requests with a concurrency of 1
	resolver := merge.Requests(requests).WithConcurrency(1)
	err := resolver.Merge()
	if err != nil {
		log.Fatalln(err)	}
	
	// the results channel now contains all our results
	close(results)
	for result := range results {
		log.Println(result)	}
}
```



