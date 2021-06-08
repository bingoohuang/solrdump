package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/bingoohuang/gg/pkg/ctx"
	"github.com/bingoohuang/gg/pkg/jihe"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/rotate"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/gobars/solrdump/pester"

	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/jj"
)

func (a App) Usage() string {
	return fmt.Sprintf(`
Usage of %s (%s):
  -max int       Max number of rows (default 10)
  -q string      SOLR query (default "*:*")
  -rows int      Number of rows returned per request (default 1000)
  -server string SOLR server with index name, eg. localhost:8983/solr/example
  -version       Show version and exit
  -remove-fields Remove fields, _version_ defaulted
  -output        Output file, or http url, or noop
  -cursor        Enable cursor or not
  -v             Verbose, -vv -vvv
`, os.Args[0], a.VersionInfo())
}
func (App) VersionInfo() string { return "0.1.3" }

type App struct {
	Server       string `required:"true"`
	Q            string `val:"*:*"`
	Max          int    `val:"10"`
	Rows         int    `val:"1000"`
	Version      bool
	Cursor       bool `val:"true"`
	RemoveFields []string
	Output       []string
	Verbose      int `flag:"v" count:"true"`

	baseURL       string
	query         url.Values
	total         int
	outputFn      func(doc []byte)
	Context       context.Context
	ContextCancel context.CancelFunc
	closers       []io.Closer

	printer     *jihe.DelayChan
	linkPrinter *jihe.DelayChan
	httpPrinter *jihe.DelayChan
}

func main() {
	c, cancelFunc := ctx.RegisterSignals(context.Background())
	a := &App{Context: c, ContextCancel: cancelFunc}
	flagparse.Parse(a)

	log.Printf("started")
	start := time.Now()

	for !a.ReachedMax() {
		link := a.CreateLink()
		if a.Verbose >= 2 {
			humanLink, _ := url.QueryUnescape(link)
			a.linkPrinter.Put(fmt.Sprintf("solr query: %q", humanLink))
		}

		cursor, err := a.Dump(link)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if cursor == a.GetCursor() || a.Context.Err() != nil {
			break
		}

		a.SetCursor(cursor)
	}

	for _, c := range a.closers {
		_ = c.Close()
	}

	a.ContextCancel()

	cost := time.Since(start)
	log.Printf("process %d docs, rate %f docs/s, cost %s", a.total, float64(a.total)/cost.Seconds(), cost)
}

func (a *App) createQuery() {
	a.query = url.Values{}
	a.query.Set("q", a.Q)
	a.query.Set("sort", "id asc")
	a.query.Set("rows", fmt.Sprintf("%d", a.Rows))
	a.query.Set("fl", "")
	a.query.Set("wt", "json")
	a.SetCursor("*")
}

const cursorMark = "cursorMark"

func (a App) GetCursor() string {
	if a.Cursor {
		return a.query.Get(cursorMark)
	}

	return "na"
}
func (a *App) SetCursor(mark string) {
	if a.Cursor {
		a.query.Set(cursorMark, mark)
	} else {
		a.query.Set("start", fmt.Sprintf("%d", a.total))
	}
}
func (a App) ReachedMax() bool { return a.Max > 0 && a.total >= a.Max }

func (a *App) Dump(url string) (string, error) {
	resp, err := pester.GetContext(a.Context, url)
	if err != nil {
		return "", fmt.Errorf("http %s: %w", url, err)
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	if code >= 400 {
		b, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("resp status: %d body (%d): %s", code, len(b), string(b))
	}

	var r SolrResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&r); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	for _, doc := range r.Response.Docs {
		for _, fl := range a.RemoveFields {
			doc, _ = jj.DeleteBytes(doc, fl, jj.SetOptions{ReplaceInPlace: true})
		}
		a.outputFn(doc)
	}

	docs := len(r.Response.Docs)
	if docs > 0 {
		a.total += docs
		a.printer.Put(fmt.Sprintf("fetched %d/%d docs", a.total, r.Response.NumFound))
	}

	if a.Cursor {
		return r.NextCursorMark, nil
	}

	return ss.If(docs < a.Rows, "na", ""), nil
}

func (a *App) PostProcess() {
	var err error

	if a.baseURL, err = rest.FixURI(a.Server); err != nil {
		log.Fatalf("bad server %s, err: %v", a.Server, err)
	}

	if a.Max > 0 && a.Max < a.Rows {
		a.Rows = a.Max
	}

	a.createQuery()

	if len(a.RemoveFields) == 0 {
		a.RemoveFields = []string{"_version_"}
	}
	interval := time.Duration(ss.Ifi(a.Verbose >= 1, 5, 10)) * time.Second
	a.printer = jihe.NewDelayChan(a.Context, func(i interface{}) { log.Printf(i.(string)) }, interval)
	a.linkPrinter = jihe.NewDelayChan(a.Context, func(i interface{}) { log.Printf(i.(string)) }, interval)
	a.httpPrinter = jihe.NewDelayChan(a.Context, func(i interface{}) { log.Printf(i.(string)) }, interval)

	a.outputFn = a.createOutputFn()

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func (a *App) createOutputFn() func(doc []byte) {
	if len(a.Output) == 0 {
		return func(doc []byte) { fmt.Println(string(doc)) }
	}

	if len(a.Output) == 1 && a.Output[0] == "noop" {
		return func(doc []byte) {}
	}

	var fns []func(doc []byte)
	for _, out := range a.Output {
		if uri, ok := rest.MaybeURL(out); ok {
			fns = append(fns, func(doc []byte) {
				outputHttp(uri, a.Verbose, doc, a.httpPrinter)
			})
		} else {
			p := osx.ExpandHome(out)
			w := rotate.NewQueueWriter(a.Context, p, 1000, false)
			a.closers = append(a.closers, w)

			fns = append(fns, func(doc []byte) {
				w.Send(string(doc)+"\n", true)
			})
		}
	}

	return func(doc []byte) {
		for _, f := range fns {
			f(doc)
		}
	}
}

type JsonValue struct {
	Value []byte
}

func (j *JsonValue) GetValue(name string) interface{} {
	result := jj.GetBytes(j.Value, name)
	return result.String()
}

func outputHttp(uri0 string, verbose int, doc []byte, printer *jihe.DelayChan) {
	// 从doc中提取并替换uri中的变量
	uri := vars.EvalSubstitute(uri0, &JsonValue{Value: doc})
	if verbose >= 1 && uri != uri0 {
		printer.Put(fmt.Sprintf("evaluated uri: %s", uri))
	}

	start := time.Now()
	resp, err := pester.Post(uri, "application/json; charset=utf-8", bytes.NewReader(doc))
	cost := time.Since(start)
	if err != nil {
		log.Printf("sent to %s error %v", uri, err)
		return
	}

	if verbose >= 2 {
		body, _ := rest.ReadCloseBody(resp)
		printer.Put(fmt.Sprintf("sent cost: %s status: %d, body: %s", cost, resp.StatusCode, body))
	} else if verbose >= 1 {
		_ = rest.DiscardCloseBody(resp)
		printer.Put(fmt.Sprintf("sent cost: %s status: %d", cost, resp.StatusCode))
	}
}

func (a App) CreateLink() string {
	return fmt.Sprintf("%s/select?%s", a.baseURL, a.query.Encode())
}

// SolrResponse is a SOLR response.
type SolrResponse struct {
	//Header   Header `json:"header"`
	Response       Response `json:"response"`
	NextCursorMark string   `json:"nextCursorMark"`
}

type Response struct {
	NumFound int               `json:"numFound"`
	Start    int               `json:"start"`
	Docs     []json.RawMessage `json:"docs"` // dependent on SOLR schema
}

type Header struct {
	Status int `json:"status"`
	QTime  int `json:"QTime"`
	Params struct {
		Query      string `json:"q"`
		CursorMark string `json:"cursorMark"`
		Sort       string `json:"sort"`
		Rows       string `json:"rows"`
	} `json:"params"`
}
