package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	upstream string
)

type PromResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

type Datapoint struct {
	Time  time.Time
	Value float64
}

type PromResult struct {
	Metric string
	Values []Datapoint
}

func Query(q string, start time.Time, end time.Time, step int) ([]PromResult, error) {
	body := url.Values{}
	body.Set("query", q)
	body.Set("start", fmt.Sprintf("%d", start.Unix()))
	body.Set("end", fmt.Sprintf("%d", end.Unix()))
	body.Set("step", fmt.Sprintf("%d", step))
	resp, err := http.Post(fmt.Sprintf("%s/api/v1/query_range", upstream),
		"application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Received %d response from upstream", resp.StatusCode)
	}

	var data PromResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Data.ResultType != "matrix" {
		return nil, fmt.Errorf("result type isn't of type matrix: %s",
			data.Data.ResultType)
	}

	if len(data.Data.Result) == 0 {
		return nil, fmt.Errorf("No data")
	}

	var results []PromResult
	for _, res := range data.Data.Result {
		r := PromResult{}
		r.Metric = metricName(res.Metric)

		var values []Datapoint
		for _, vals := range res.Values {
			timestamp := vals[0].(float64)
			value := vals[1].(string)
			fv, _ := strconv.ParseFloat(value, 64)
			values = append(values, Datapoint{
				time.Unix(int64(timestamp), 0),
				fv,
			})
		}
		r.Values = values

		results = append(results, r)
	}
	return results, nil
}

func metricName(metric map[string]string) string {
	if len(metric) == 0 {
		return "{}"
	}

	out := ""
	var inner []string
	for key, value := range metric {
		if key == "__name__" {
			out = value
			continue
		}
		inner = append(inner, fmt.Sprintf(`%s="%s"`, key, value))
	}

	if len(inner) == 0 {
		return out
	}

	sort.Slice(inner, func(i, j int) bool {
		return inner[i] < inner[j]
	})

	return out + "{" + strings.Join(inner, ",") + "}"
}

func main() {
	plotutil.DefaultDashes = [][]vg.Length{{}}

	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s server\n", os.Args[0])
		os.Exit(1)
	}
	upstream = os.Args[1]
	router := chi.NewRouter()
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)

	router.Get("/chart.svg", func(w http.ResponseWriter, r *http.Request) {
		args := r.URL.Query()
		var query string
		if q, ok := args["query"]; !ok {
			w.WriteHeader(400)
			w.Write([]byte("Expected ?query=... parameter"))
			return
		} else {
			query = q[0]
		}

		start := time.Now().Add(-24 * 60 * time.Minute)
		end := time.Now()
		if s, ok := args["since"]; ok {
			d, _ := time.ParseDuration(s[0])
			start = time.Now().Add(-d)
		}
		if u, ok := args["until"]; ok {
			d, _ := time.ParseDuration(u[0])
			end = time.Now().Add(-d)
		}

		width := 12*vg.Inch
		height := 6*vg.Inch
		if ws, ok := args["width"]; ok {
			w, _ := strconv.ParseFloat(ws[0], 32)
			width = vg.Length(w)*vg.Inch
		}
		if hs, ok := args["height"]; ok {
			h, _ := strconv.ParseFloat(hs[0], 32)
			height = vg.Length(h)*vg.Inch
		}

		// Undocumented option
		var legend string
		if l, ok := args["legend"]; ok {
			legend = l[0]
		}

		// Set step so that there's approximately 25 data points per inch
		step := int(end.Sub(start).Seconds() / (25 * float64(width / vg.Inch)))
		if s, ok := args["step"]; ok {
			d, _ := strconv.ParseInt(s[0], 10, 32)
			step = int(d)
		}
		_, stacked := args["stacked"]

		data, err := Query(query, start, end, step)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprintf("%v", err)))
			return
		}

		p, err := plot.New()
		if err != nil {
			panic(err)
		}
		if t, ok := args["title"]; ok {
			p.Title.Text = t[0]
		}
		p.X.Label.Text = "Time"
		p.X.Tick.Marker = dateTicks{start, end}
		if ms, ok := args["max"]; ok {
			m, _ := strconv.ParseFloat(ms[0], 64)
			p.Y.Max = m
		}

		p.Y.Tick.Marker = humanTicks{}
		if ms, ok := args["min"]; ok {
			m, _ := strconv.ParseFloat(ms[0], 64)
			p.Y.Min = m
		}
		p.Legend.Top = true

		sums := make([]float64, len(data[0].Values))

		plotters := make([]plot.Plotter, len(data))
		var nextColor int
		colors := plotutil.SoftColors
		for i, res := range data {
			var points plotter.XYs
			for j, d := range res.Values {
				value := d.Value
				if stacked {
					value += sums[j]
				}
				points = append(points, plotter.XY{
					float64(d.Time.Unix()),
					value,
				})
				sums[j] += d.Value
			}

			l, _, err := plotter.NewLinePoints(points)
			if err != nil {
				w.WriteHeader(400)
				w.Write([]byte(fmt.Sprintf("%v", err)))
				return
			}
			if stacked {
				l.FillColor = colors[nextColor]
				if i != len(data) - 1 {
					l.Color = color.RGBA{0, 0, 0, 0}
				}
			} else {
				l.Color = colors[nextColor]
			}
			nextColor += 1
			if nextColor >= len(colors) {
				nextColor = 0
			}
			plotters[i] = l
			if legend != "" {
				p.Legend.Add(legend, l)
			} else {
				p.Legend.Add(res.Metric, l)
			}
		}
		for i := len(plotters) - 1; i >= 0; i-- {
			p.Add(plotters[i])
		}

		writer, err := p.WriterTo(width, height, "svg")
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprintf("%v", err)))
			return
		}

		w.Header().Add("Content-Type", "image/svg+xml")
		writer.WriteTo(w)
	})

	addr := ":8142"
	if len(os.Args) > 2 {
		addr = os.Args[2]
	}
	log.Printf("Listening on %s", addr)
	http.ListenAndServe(addr, router)
}

type dateTicks struct {
	Start time.Time
	End   time.Time
}

// Ticks computes the default tick marks, but inserts commas
// into the labels for the major tick marks.
func (dt dateTicks) Ticks(min, max float64) []plot.Tick {
	fmt := "15:04:05"
	if dt.End.Sub(dt.Start).Hours() >= 24 {
		fmt = "Jan 2 15:04:05"
	}

	tks := plot.DefaultTicks{}.Ticks(min, max)
	for i, t := range tks {
		if t.Label == "" { // Skip minor ticks, they are fine.
			continue
		}
		d, _ := strconv.ParseFloat(t.Label, 64)
		tm := time.Unix(int64(d), 0)
		tks[i].Label = tm.Format(fmt)
	}
	return tks
}

type humanTicks struct{}

func (ht humanTicks) Ticks(min, max float64) []plot.Tick {
	tks := plot.DefaultTicks{}.Ticks(min, max)
	for i, t := range tks {
		if t.Label == "" { // Skip minor ticks, they are fine.
			continue
		}
		d, _ := strconv.ParseFloat(t.Label, 64)
		tks[i].Label = humanize.SI(d, "")
	}
	return tks
}
