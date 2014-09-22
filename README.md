par
=====

par is a Go library to process calls with the following characteristics:

* calls will be processed in parallel 
* parallelism may be optionally bounded
* leverages Google's Context library to allow calls to be timed our or actively canceled

## Example - Coin Toss

Let's start off with a Coin Toss example.  Suppose we would like to flip a coin N times in parallel.  Using par, we could write the following:

```go
import (
  "code.google.com/p/go.net/context"
  "github.com/savaki/par"
  "math/rand"
)

func flipCoin(results chan CoinFlip) par.RequestFunc {
  return func(context.Context) error {
    if rand.Intn(2) == 0 {
      results <- Heads
    } else {
      results <- Tails
    }
    return nil
  }
}

func simple() {
  flips := 10

  // 1. create a channel to hold your results
  results := make(chan CoinFlip, flips)

  // 2. create a channel of requests
  requests := make(chan par.RequestFunc, flips)
  for flip := 0; flip < flips; flip = flip + 1 {
    requests <- flipCoin(results)
  }
  close(requests)

  // 3. execute the flips in parallel
  _ = par.Requests(requests).Do()

  // 4. results channel now has your results and can be ranged over
}
```

## Find the weather in 3 cities with a concurrency of 1

Seems like a lot of work to execute 10 parallel flips.  But let's take a more realistic scenario.  Suppose we want to check the weather in three cities.  Using [openweathermap](http://openweathermap.org), we can query the weather in any one city.  For our three cities, querying them one at a time is slow.  par allows us to query these cities in parallel.  As follows:


```
package main

import (
	"code.google.com/p/go.net/context"
	"github.com/savaki/par"
	"github.com/savaki/openweathermap"
	"log"
)

func ok(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func find(city string, responses chan *openweathermap.Forecast) par.RequestFunc {
	return func(ctx context.Context) error {
		forecast, err := openweathermap.WithContext(ctx).ByCityName(city)
		ok(err)
		responses <- forecast
		return nil
	}
}

func main() {
	// create a channel to capture our results

	forecasts := make(chan *openweathermap.Forecast, 3)

	// create our channel of requests

	requests := make(chan par.RequestFunc, 3)
	requests <- find("San Francisco", forecasts)
	requests <- find("Oakland", forecasts)
	requests <- find("Berkeley", forecasts)
	close(requests) // important to remember to close the channel

	// execute the requests with a concurrency of 1

	resolver := par.Requests(requests).WithConcurrency(1)
	err := resolver.Do()
	ok(err)

	// the forecasts channel now contains all our forecasts

	close(forecasts)
	cities := map[string]*openweathermap.Forecast{}
	for forecast := range forecasts {
		cities[forecast.Name] = forecast
	}
}
```

Here we're using a [go client for openweathermap](http://github.com/savaki/openweathermap) to get the weather by city.  Since we want to be a well behaved netizen, we don't want to flood their service, so we'll execute 1 query at a time.

### Results

You can see from the results, that we wait until we receive a query before sending the next one. 

```
22:00:07.800 349us  349us  par - request: 0
22:00:08.393 593ms  593ms  openweathermap - http - ok
22:00:08.393 593ms  593ms  par - received - 0
22:00:08.393 3us    3us    par - request: 1
22:00:08.684 290ms  290ms  openweathermap - http - ok
22:00:08.684 290ms  290ms  par - received - 1
22:00:08.684 3us    3us    par - request: 2
22:00:08.869 184ms  184ms  openweathermap - http - ok
22:00:08.869 184ms  184ms  par - received - 2
22:00:08.869 5us    5us    par - finished
```

## Example - Find the weather in 3 cities with a concurrency of 3

As we've stated before, this can be slow for the end user.  So now, we've talked to openweathermap and set up an account and now our concurrent limit has been upped to 3.  Wonderful.  Let's first look at what code change would be required.  Here we add ```.WithConcurrency(3)``` to indicate make at most three calls at a time.

```
	resolver := par.Requests(requests).WithConcurrency(3)
	err := resolver.Do()
	ok(err)
```

### Results

Looking at our results we can see what calls 0, 1, 2 go through immediately since our limit is 3.  Great.

```
22:00:58.584 155us  155us  par - request: 0
22:00:58.584 40us   40us   par - request: 1
22:00:58.584 8us    8us    par - request: 2
22:00:58.781 196ms  196ms  openweathermap - http - ok
22:00:58.781 196ms  196ms  par - received - 0
22:00:58.786 5ms    5ms    openweathermap - http - ok
22:00:58.786 5ms    5ms    par - received - 1
22:00:58.787 1ms    1ms    openweathermap - http - ok
22:00:58.787 1ms    1ms    par - received - 2
22:00:58.787 4us    4us    par - finished
```

## Example - Find the weather in 3 cities with a concurrency of 2 and timeout of 250ms

Now let's suppose we can't afford 3 concurrent calls with this service provide and we need to drop down to two.  We still want to provide a responsive service to our users, so let's set an upper bound on how long the call can take via ```.DoWithContext(ctx)```  the excellent [context](http://blog.golang.org/context) library is courtesy of Google.

```
	resolver := par.Requests(requests).WithConcurrency(2)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	err := resolver.DoWithContext(ctx)
```

### Result

So now we see that we're only making two calls at a time and that we didn't finish our third call before the timeout happened.

```
22:06:14.061 381us  381us  par - request: 0
22:06:14.061 422us  422us  par - request: 1
22:06:14.237 176ms  176ms  openweathermap - http - ok
22:06:14.237 175ms  175ms  par - received - 0
22:06:14.237 2us    2us    par - request: 2
22:06:14.248 11ms   11ms   openweathermap - http - ok
22:06:14.249 11ms   11ms   par - received - 1
22:06:14.312 63ms   63ms   par - timeout
```

## Example - Find the weather in 3 cities with an unbounded pool, redundancy of 2 and timeout of 200ms

Sometimes, it's not about trying to fit within the bounds of an api agreement.  Sometimes, we want to provide the best experience possible for our users.  Every now and then, when you invoke a service, you get a slow response.  Maybe it's one in ten times you get 500ms of latency.  90% of the time, it's fine.  But 10% of the time, ugh.  Now instead of a single call, if you made two redundant calls and took the fast responses, your 1 in 10 chance now drops to 1 in 100.  Much better!  

The more concurrent calls to other services you make, the more likely you'll want to make redundant calls to minimize the latency issues that will arise.  Let's take a look at our example again.  Here we'd like to use unbounded parallelism (make as many concurrent calls as you like) with a redundancy of 2 (make each city's weather request twice).  We can do that via the ```.WithRedundancy(2)``` call.

Here's the core element of code.

```
	resolver := par.Requests(requests).WithRedundancy(2)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := resolver.DoWithContext(ctx)
```

### Result

In the results, you can see we're making multiple calls for each of our cities, but in spite of that, we're still not able to finish in the 200ms response timeout window.

```
22:12:41.270 649us  649us  par - request: 0
22:12:41.270 45us   45us   par - request: 1
22:12:41.270 10us   10us   par - request: 2
22:12:41.270 13us   13us   par - request: 0
22:12:41.270 11us   11us   par - request: 1
22:12:41.270 10us   9us    par - request: 2
22:12:41.453 183ms  183ms  openweathermap - http - ok
22:12:41.453 183ms  183ms  par - received - 1
22:12:41.455 2ms    2ms    openweathermap - http - ok
22:12:41.455 2ms    2ms    par - received - 2
22:12:41.457 1ms    1ms    openweathermap - http - ok
22:12:41.457 1ms    1ms    par - received - 1
22:12:41.458 1ms    1ms    openweathermap - http - ok
22:12:41.463 5ms    5ms    openweathermap - http - ok
22:12:41.466 2ms    2ms    openweathermap - http - ok
22:12:41.470 13ms   13ms   par - timeout
```

## Debugging output

To see the debug output for yourself, execute the following in your shell:

```
export DEBUG=\*
```
