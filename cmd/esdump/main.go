package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/jj"
	"github.com/gobars/solrdump/pester"
)

func (Arg) VersionInfo() string { return "0.1.2 2021-06-10 13:48:53" }

func (a Arg) Usage() string {
	return fmt.Sprintf(`
Usage of %s (%s):
  -es    string    Elasticsearch address, default 127.0.0.1:9202
  -index string    Index name, default zz
  -type  string    Index type, default _doc
  -scroll string   Scroll time ttl, default 1m
  -max      int    Max docs to dump, default 10000
  -query string    Query json, like {"size":10000,"_source":["holderNum"]}
  -version         Show version and exit
  -filter string   Filter expression, like hits.hits.#._source.holderIdentityNum.0, default hits.hits.#._source
  -out             Out, empty/stdout to stdout, else to badger path.
  -v               Verbose, -vv -vvv
  -view-badger int Print Badger max kvs and exit
`, os.Args[0], a.VersionInfo())
}

type Arg struct {
	Es      string `val:"127.0.0.1:9200"`
	Index   string `val:"zz"`
	Type    string `val:"_doc"`
	Scroll  string `val:"1m"`
	Max     int    `val:"10000"`
	Query   string
	Filter  string `val:"hits.hits.#._source"`
	Out     string
	Version bool
}

func main() {
	a := &Arg{}
	flagparse.Parse(a)

	c, ctxCancel := sigx.RegisterSignals(nil)
	defer ctxCancel()

	out := a.createOut()
	defer out.Close()

	// uri := `http://192.168.126.5:9202/license/docs/_search?scroll=1m`
	uri, _ := rest.NewURL(a.Es).Paths(a.Index, a.Type, `/_search`).Query("scroll", a.Scroll).Build()

	r, tim := Post(uri, []byte(a.Query))
	cost := tim

	scrollUri, _ := rest.NewURL(a.Es).Paths("/_search/scroll").Build()
	payloadTemplate := []byte(`{"scroll_id":"","scroll":"` + a.Scroll + `"}`)
	totalHits := 0
	var scrollPayload []byte

	for {
		hits := 0
		body, _ := rest.ReadCloseBody(r)
		jj.GetBytes(body, a.Filter).ForEach(func(_, c jj.Result) bool {
			hits++
			if err := out.Output(c.String()); err != nil {
				log.Printf("failed to out result: %v", err)
				return false
			}
			return true
		})

		totalHits += hits
		if hits > 0 {
			log.Printf("total hists %d, cost %s", totalHits, cost)
		}

		if hits <= 0 || (a.Max > 0 && totalHits >= a.Max) || c.Err() != nil {
			break
		}

		if len(scrollPayload) == 0 {
			v := jj.GetBytes(body, "_scroll_id")
			scrollPayload, _ = jj.SetBytes(payloadTemplate, "scroll_id", v.String())
		}

		r, tim = Post(scrollUri, scrollPayload)
		cost += tim
	}
}

func Post(url string, payload []byte) (*http.Response, time.Duration) {
	start := time.Now()
	r, err := pester.Post(url, rest.ContentTypeJSON, bytes.NewReader(payload))
	cost := time.Since(start)
	if err != nil {
		panic(err)
	}

	return r, cost
}

func (a *Arg) createOut() Out {
	// if a.Out == "" || a.Out == "stdout" {
	return &Stdout{}
}

type Out interface {
	io.Closer
	Output(doc string) error
}

type Stdout struct {
	Index uint64
}

func (s *Stdout) Close() error { return nil }
func (s *Stdout) Output(doc string) error {
	s.Index++
	log.Printf("%010d:%s", s.Index, doc)
	return nil
}
