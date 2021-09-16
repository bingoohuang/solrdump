package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/gobars/solrdump/pester"
)

func (a Arg) createSolrLink() string {
	return fmt.Sprintf("%s/select?%s", a.baseURL, a.query.Encode())
}

func (a *Arg) prepareSolrQuery() {
	a.query = url.Values{}
	// https://solr.apache.org/guide/6_6/common-query-parameters.html
	a.query.Set("q", a.Q)
	a.query.Set("sort", "id asc")
	a.query.Set("rows", fmt.Sprintf("%d", a.Rows))
	a.query.Set("fl", a.Fl)   // Field List
	a.query.Set("wt", "json") // Specifies the Response Writer to be used to format the query response.
	a.SetCursor("*")
}

func (a *Arg) SolrDump(url string) (string, error) {
	resp, err := pester.GetContext(a.Context, url)
	if err != nil {
		return "", fmt.Errorf("http %s: %w", url, err)
	}
	defer rest.DiscardCloseBody(resp)

	code := resp.StatusCode
	if code >= 400 {
		b, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("resp status: %d body (%d): %s", code, len(b), string(b))
	}

	var r SolrResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	a.ResponseCh <- r.Response

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
	// Header   Header `json:"header"`
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
