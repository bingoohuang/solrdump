package main

import (
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"github.com/bingoohuang/gg/pkg/delay"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/golog"

	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/osx"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/rotate"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/jj"
)

//go:embed initassets
var initAssets embed.FS

func main() {
	c, cancelFunc := sigx.RegisterSignals(context.Background())
	a := &Arg{Context: c, outputWg: &sync.WaitGroup{}}
	flagparse.Parse(a,
		flagparse.AutoLoadYaml("c", "solrdump.yml"),
		flagparse.ProcessInit(&initAssets))
	defer golog.Setup().OnExit()
	log.Printf("started with config: %+v created", a)
	start := time.Now()

	a.StartOutput()

	wal, err := jj.WalOpen(".cursorMarks", &jj.WalOptions{LogFormat: jj.JSONFormat})
	if err != nil {
		log.Fatalf("failed to open cursor wal: %v", err)
	}
	defer wal.Close()

	if err := a.readLastCursor(wal); err != nil {
		log.Fatalf("failed to read last cursor: %v", err)
	}

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

		logCursor(wal, cursor)
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
	log.Printf("process %d docs, rate %f docs/s, cost %s, dups: %d",
		a.total, float64(a.total)/cost.Seconds(), cost, atomic.LoadUint32(&totalDups))
}

func (a *Arg) readLastCursor(wal *jj.WalLog) (err error) {
	if a.Force { // Force a new query from cursorMark = "*"
		return nil
	}

	lastIndex, err := wal.LastIndex()
	if err != nil {
		return err
	}
	if lastIndex <= 0 {
		return nil
	}

	if data, err := wal.Read(lastIndex); err != nil {
		return err
	} else {
		a.SetCursor(string(data))
	}
	return nil
}

func logCursor(wal *jj.WalLog, cursor string) {
	first, last, err := wal.Index()
	if err != nil {
		log.Fatalf("get cursor wal last index, error: %v", err)
	}
	if err := wal.Write(last+1, []byte(cursor)); err != nil {
		log.Fatalf("write cursor wal, error: %v", err)
	}

	if last-first > 1000 {
		wal.TruncateFront(last - 10)
	}
}

func (a *Arg) StartOutput() {
	a.outputWg.Add(1)
	go func() {
		defer a.outputWg.Done()

		for resp := range a.ResponseCh {
			a.processResponse(resp)
		}
	}()
}

func (a *Arg) processResponse(resp Response) {
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

var totalDups uint32

func (a *Arg) PostProcess() {
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
		printer := delay.NewChan(a.Context, func(_, i interface{}) {
			log.Printf(i.(string))
		}, interval)
		a.closers = append(a.closers, printer)
		a.printer = printer
	} else {
		a.printer = &LogPrinter{}
	}
	a.ResponseCh = make(chan Response, 1)
	a.outputFn = a.createOutputFn()

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func (a *Arg) createOutputFn() func(doc []byte) {
	if len(a.Output) == 0 {
		return func(doc []byte) { fmt.Println(string(doc)) }
	}
	if len(a.Output) == 1 && a.Output[0] == "noop" {
		return func(doc []byte) {}
	}

	var fns []func(doc []byte)
	for _, out := range a.Output {
		var fn func(doc []byte)
		const prefix = "find-duplicate:"
		if strings.HasPrefix(out, prefix) {
			byKey := out[len(prefix):]
			var lastValue string

			lastDups := 0

			fn = func(doc []byte) {
				value := jj.GetBytes(doc, byKey).String()
				if lastValue == value {
					lastDups++
					if lastDups == 1 {
						atomic.AddUint32(&totalDups, 1)
					}
					fmt.Printf("%s\n", doc)
				} else {
					lastDups = 0
				}
				lastValue = value
			}
		} else {
			if uri, ok := rest.MaybeURL(out); ok {
				if fn = a.createBulkOutput(uri); fn == nil {
					fn = func(doc []byte) { outputHttp(uri, doc, a.Verbose, a.printer) }
				}
			} else {
				w := rotate.NewQueueWriter(osx.ExpandHome(out), rotate.WithContext(a.Context),
					rotate.WithOutChanSize(1000), rotate.WithAllowDiscard(false))
				a.closers = append(a.closers, w)
				fn = func(doc []byte) { w.Send(string(doc)+"\n", true) }
			}
		}
		fns = append(fns, fn)
	}

	return func(doc []byte) {
		for _, f := range fns {
			f(doc)
		}
	}
}
