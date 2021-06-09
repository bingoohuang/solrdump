package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bingoohuang/gg/pkg/ctx"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/jihe"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/rotate"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/jj"
)

func main() {
	c, cancelFunc := ctx.RegisterSignals(context.Background())
	a := &App{Context: c, outputWg: &sync.WaitGroup{}}
	flagparse.Parse(a)

	log.Printf("started")
	start := time.Now()

	a.StartOutput()

	for !a.ReachedMax() {
		link := a.createSolrLink()
		if a.Verbose >= 2 {
			humanLink, _ := url.QueryUnescape(link)
			a.printer.PutKey("link", fmt.Sprintf("solr query: %q", humanLink))
		}

		cursor, err := a.SolrDump(link)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if cursor == a.GetCursor() || a.Context.Err() != nil {
			break
		}

		a.SetCursor(cursor)
	}

	close(a.ResponseCh)
	a.outputWg.Wait()

	for _, c := range a.closers {
		_ = c.Close()
	}

	cancelFunc()
	cost := time.Since(start)
	log.Printf("process %d docs, rate %f docs/s, cost %s", a.total, float64(a.total)/cost.Seconds(), cost)
}

func (a *App) StartOutput() {
	a.outputWg.Add(1)
	go func() {
		defer a.outputWg.Done()

		for resp := range a.ResponseCh {
			a.processResponse(resp)
		}
	}()
}

func (a *App) processResponse(resp Response) {
	for _, doc := range resp.Docs {
		for _, v := range a.RemoveFields {
			if vv, err := jj.DeleteBytes(doc, v, jj.SetOptions{ReplaceInPlace: true}); err != nil {
				log.Printf("failed to delete %s from doc %s", v, doc)
			} else {
				doc = vv
			}
		}
		a.outputFn(doc)
	}
}

func (a *App) PostProcess() {
	var err error

	if a.baseURL, err = rest.FixURI(a.Server); err != nil {
		log.Fatalf("bad server %s, err: %v", a.Server, err)
	}

	if a.Max > 0 && a.Max < a.Rows {
		a.Rows = a.Max
	}

	a.prepareSolrQuery()

	if len(a.RemoveFields) == 0 {
		a.RemoveFields = []string{"_version_"}
	}

	if a.Verbose <= 2 {
		interval := time.Duration(ss.Ifi(a.Verbose >= 1, 5, 10)) * time.Second
		printer := jihe.NewDelayChan(a.Context, func(i interface{}) { log.Printf(i.(string)) }, interval)
		a.closers = append(a.closers, printer)
		a.printer = printer
	} else {
		a.printer = &LogPrinter{}
	}
	a.ResponseCh = make(chan Response, 1)
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
			fn := a.createBulkOutput(uri)
			if fn == nil {
				fn = func(doc []byte) { outputHttp(uri, doc, a.Verbose, a.printer) }
			}

			fns = append(fns, fn)
		} else {
			w := rotate.NewQueueWriter(a.Context, osx.ExpandHome(out), 1000, false)
			a.closers = append(a.closers, w)

			fns = append(fns, func(doc []byte) { w.Send(string(doc)+"\n", true) })
		}
	}

	return func(doc []byte) {
		for _, f := range fns {
			f(doc)
		}
	}
}
