package main

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/gg/pkg/jsoni"
	"github.com/bingoohuang/gg/pkg/jsoni/extra"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/strcase"
	"github.com/go-resty/resty/v2"
)

func (a Arg) createSolrLink() string {
	return fmt.Sprintf("%s/select?%s", a.baseURL, a.query.Encode())
}

func (a *Arg) prepareSolrQuery() {
	a.query = url.Values{}
	// https://solr.apache.org/guide/6_6/the-standard-query-parser.html#the-standard-query-parser
	// field:[* TO 100]   field:[100 TO *]
	// datefield:[1976-03-06T23:59:59.999Z TO *]
	// datefield:[2000-11-01 TO 2014-12-01]
	// -inStock:false finds all field values where inStock is not false
	// -field:[* TO *] finds all documents without a value for field
	a.query.Set("q", a.Q)
	// https://solr.apache.org/guide/6_6/common-query-parameters.html
	if a.Sort != "" && !ss.HasSuffix(strings.ToLower(a.Sort), "asc", "desc") {
		a.Sort += " asc"
	}
	a.query.Set("sort", ss.Or(a.Sort, "id asc"))
	a.query.Set("rows", fmt.Sprintf("%d", a.Rows))
	a.query.Set("fl", a.Fl)   // Field List
	a.query.Set("wt", "json") // Specifies the Response Writer to be used to format the query response.
	a.query.Set("omitHeader", "true")
	a.SetCursor("*")
}

// Jsoni tries to be 100% compatible with standard library behavior
var Jsoni = jsoni.Config{
	EscapeHTML: true,
}.Froze()

func init() {
	Jsoni.RegisterExtension(&extra.NamingStrategyExtension{Translate: strcase.ToCamelLower})
}

// Create a Resty Client
var restyClient = resty.New()

func (a *Arg) SolrDump(url string) (string, error) {
	start := time.Now()
	resp, err := restyClient.R().SetContext(a.Context).Get(url)
	if err != nil {
		return "", fmt.Errorf("http %s: %w", url, err)
	}

	b := resp.Body()
	if code := resp.StatusCode(); code >= 400 {
		return "", fmt.Errorf("resp status: %d body (%d): %s", code, len(b), string(b))
	}

	var r SolrResponse
	if err := Jsoni.NewDecoder(bytes.NewReader(b)).Decode(a.Context, &r); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	a.ResponseCh <- r.Response

	docs := len(r.Response.Docs)
	if docs > 0 {
		a.total += docs
		a.printer.Put(fmt.Sprintf("fetched %d/%d docs, cost %s, dups: %d",
			a.total, r.Response.NumFound, time.Since(start), atomic.LoadUint32(&totalDups)))
	}

	if a.Cursor {
		return r.NextCursorMark, nil
	}

	return ss.If(docs < a.Rows, "na", ""), nil
}

const cursorMark = "cursorMark"

func (a Arg) GetCursor() string {
	if a.Cursor {
		return a.query.Get(cursorMark)
	}

	return "na"
}

func (a *Arg) SetCursor(mark string) {
	if a.Cursor {
		a.query.Set(cursorMark, mark)
	} else {
		a.query.Set("start", fmt.Sprintf("%d", a.total))
	}
}
func (a Arg) ReachedMax() bool { return a.Max > 0 && a.total >= a.Max }

// SolrResponse is a SOLR response.
type SolrResponse struct {
	NextCursorMark string   `json:"nextCursorMark"`
	Response       Response `json:"response"`
}

type Response struct {
	Docs     []jsoni.RawMessage `json:"docs"` // dependent on SOLR schema
	NumFound int                `json:"numFound"`
	Start    int                `json:"start"`
}
