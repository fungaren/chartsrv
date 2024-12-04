# chartsrv

chartsrv is a dead-simple web application which runs [Prometheus][0] queries and
charts the result as an SVG. You make a dashboard by putting a lot of these
images into a web page. You make it refresh by adding
`<meta id="refresh" http-equiv="refresh" content="5">` to the header. Ta-da!

[0]: https://prometheus.io/

![Live graph from metrics.sr.ht](https://metrics.sr.ht/chart.svg?title=Build%20worker%20load%20average&query=avg_over_time%28node_load15%7Binstance%3D~%22cirno%5B0-9%5D%2B.sr.ht%3A80%22%7D%5B1h%5D%29&max=64&since=336h&stacked&step=10000&height=3&width=10)

This is a live graph from [metrics.sr.ht](https://metrics.sr.ht)

## Running the daemon

```
$ go run main.go https://prometheus.example.org
Listening on :8142
```

Forward `/chart.svg` to this address with your favorite reverse proxy. If you
want to listen to some other port, pass a second argument like `:1337`.

## Usage

Create a URL like `https://chartsrv.example.org/chart.svg?query=...&args...` and
set the query parameters as appropriate:

- **query**: required. Prometheus query to execute.
- **title**: chart title
- **stacked**: set to create an area chart instead of a line chart
- **since**, **until**: [time.ParseDuration][1] to set distance in the past to
  start charting from or until
- **width**, **height**: adjust chart dimensions in inches
- **step**: number of seconds between data points
- **min**, **max**: Y axis limits
- **label**: template for the chart legend in Go's text/template format. All
values provided by prometheus as tags are available - to label each matching
result by the instance, for instance, use `{{.instance}}`

[1]: https://golang.org/pkg/time/#ParseDuration
