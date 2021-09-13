package main

import (
	"bytes"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bingoohuang/gg/pkg/vars"
)

func (a *Arg) createBulkOutput(uri string) func(doc []byte) {
	if !strings.Contains(uri, "/_bulk") {
		return nil
	}

	// support es bulk mode
	docCh := make(chan []byte, a.Bulk)
	fn := func(doc []byte) { docCh <- doc }

	var wg sync.WaitGroup
	wg.Add(1)
	go a.elasticSearchBulk(uri, docCh, &wg)

	a.closers = append(a.closers, closeFn(func() { close(docCh); wg.Wait() }))

	return fn
}

func (a *Arg) elasticSearchBulk(uri string, docCh chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	u, _ := url.Parse(uri)
	query := u.Query()
	routing := query.Get("routing")
	var routingExpr vars.Subs
	if routing != "" {
		query.Del("routing")
		routingExpr = vars.ParseExpr(routing)
	}
	u.RawQuery = query.Encode()
	uri = u.String()

	for {
		b, ok := numOrTicker(docCh, routingExpr, a.Bulk)
		if b.Len() > 0 {
			outputHttp(uri, b.Bytes(), a.Verbose, a.printer)
		}
		if !ok {
			return
		}
	}
}

func numOrTicker(docCh chan []byte, routingExpr vars.Subs, batchNum int) (*bytes.Buffer, bool) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	num := 0
	b := &bytes.Buffer{}

	for {
		select {
		case doc, ok := <-docCh:
			if !ok {
				return b, false
			}
			if len(routingExpr) > 0 {
				routing := routingExpr.Eval(&JsonValue{Doc: doc}).(string)
				b.Write([]byte(`{"index":{"_type":"docs","_routing":"` + routing + `"}}`))
			} else {
				b.Write([]byte(`{"index":{"_type":"docs"}}`))
			}

			b.Write([]byte("\n"))
			b.Write(doc)
			b.Write([]byte("\n"))
			if num++; num >= batchNum {
				return b, true
			}
		case <-ticker.C:
			return b, true
		}
	}
}
