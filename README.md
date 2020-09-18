# chartsrv

chartsrv is a dead-simple web application which runs [Prometheus][0] queries and
charts the result as an SVG.

[0]: https://prometheus.io/

## Running the daemon

```
$ go build -o chartsrv main.go
$ ./chartsrv https://prometheus.example.org
Listening on :8142
```

Forward `/chart.svg` to this address with your favorite reverse proxy.

## Usage

Create a URL like `https://chartsrv.example.org/chart.svg?query=...&args...` and
set the query parameters as appropriate:

- **query**: required. Prometheus query to execute.
- **title**: chart title
- **stacked**: set to create an area chart instead of a line chart
- **since**: [time.ParseDuration][1] to set distance in the past to start
  charting from
- **width**: chart width in inches
- **height**: chart height in inches
- **step**: number of seconds between data points
- **max**: maximum Y value

[1]: https://golang.org/pkg/time/#ParseDuration
