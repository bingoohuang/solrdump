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

	var routingExpr vars.Subs
	if routing := query.Get("routing"); routing != "" {
		query.Del("routing")
		routingExpr = vars.ParseExpr(routing)
	}
	var idExpr vars.Subs
	if id := query.Get("id"); id != "" {
		query.Del("id")
		idExpr = vars.ParseExpr(id)
	}

	u.RawQuery = query.Encode()
	uri = u.String()
	b := &bytes.Buffer{}

	for {
		ok := a.numOrTicker(b, docCh, routingExpr, idExpr)
		outputHttp(uri, b.Bytes(), a.Verbose, a.printer)
		if !ok {
			return
		}
	}
}

func (a *Arg) numOrTicker(b *bytes.Buffer, docCh chan []byte, routingExpr, idExpr vars.Subs) (continued bool) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	num := 0
	b.Reset()

	for {
		select {
		case <-ticker.C:
			if num > 0 {
				return true
			}

		case doc, ok := <-docCh:
			if !ok {
				return false
			}
			_id := idExpr.Eval(&JsonValue{Doc: doc}).(string) // 配置文件指定 id 字段，然后复制为 es 的 _id： https://blog.csdn.net/neweastsun/article/details/91506909
			if len(routingExpr) > 0 {
				routing := routingExpr.Eval(&JsonValue{Doc: doc}).(string)
				b.Write([]byte(`{"index":{"_type":"docs","_id":"` + _id + `","` + a.Routing + `":"` + routing + `"}}`))
			} else {
				b.Write([]byte(`{"index":{"_type":"docs","_id":"` + _id + `"}}`))
			}

			b.Write([]byte("\n"))
			b.Write(doc)
			b.Write([]byte("\n"))
			if num++; num >= a.Bulk {
				return true
			}
		}
	}
}
