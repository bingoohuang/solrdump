package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/jj"
	"github.com/sethgrid/pester"
)

func (a App) Usage() string {
	return fmt.Sprintf(`
Usage of %s (%s):
  -max int       Max number of rows (default 100)
  -q string      SOLR query (default "*:*")
  -rows int      Number of rows returned per request (default 100)
  -server string SOLR server with index name, eg. localhost:8983/solr/example
  -version       Show version and exit
`, os.Args[0], a.VersionInfo())
}
func (App) VersionInfo() string { return "0.1.1" }

type App struct {
	Server  string `usage:"SOLR server with index name, eg. localhost:8983/solr/example" required:"true"`
	Q       string `val:"*:*" usage:"SOLR query"`
	Max     int    `val:"100" usage:"Max number of rows"`
	Rows    int    `val:"100" usage:"Number of rows returned per request"`
	Version bool   `usage:"Show version and exit"`

	baseURL string
	query   url.Values
	total   int
}

func (a *App) PostProcess() {
	var err error

	if a.baseURL, err = rest.FixURI(a.Server); err != nil {
		log.Fatalf("bad server %s, err: %v", a.Server, err)
	}

	if a.Max < a.Rows {
		a.Rows = a.Max
	}

	a.query = a.createQuery()
}

func (a App) CreateLink() string {
	return fmt.Sprintf("%s/select?%s", a.baseURL, a.query.Encode())
}

func main() {
	app := &App{}
	flagparse.Parse(app)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	for !app.ReachedMax() {
		link := app.CreateLink()
		log.Println(link)

		if cursorMark := app.Dump(link); cursorMark == app.CursorMark() {
			break
		} else {
			app.SetCursorMark(cursorMark)
		}
	}
}

func (a App) createQuery() url.Values {
	v := url.Values{}
	v.Set("q", a.Q)
	v.Set("sort", "id")
	v.Set("rows", fmt.Sprintf("%d", a.Rows))
	v.Set("fl", "")
	v.Set("wt", "json")
	v.Set("cursorMark", "*")
	return v
}

func (a *App) Dump(link string) string {
	resp, err := pester.Get(link)
	if err != nil {
		log.Fatalf("http: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := ioutil.ReadAll(resp.Body)
		log.Fatalf("resp status: %d body (%d): %s", resp.Status, len(b), string(b))
	}

	var r Response
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&r); err != nil {
		log.Fatalf("decode: %s", err)
	}

	for _, doc := range r.Response.Docs {
		bytes, _ := jj.DeleteBytes(doc, "_version_", jj.SetOptions{Optimistic: true, ReplaceInPlace: true})
		fmt.Println(string(bytes))
	}

	a.total += len(r.Response.Docs)
	log.Printf("fetched %d/%d docs", a.total, r.Response.NumFound)
	return r.NextCursorMark
}

func (a App) CursorMark() string         { return a.query.Get("cursorMark") }
func (a *App) SetCursorMark(mark string) { a.query.Set("cursorMark", mark) }
func (a App) ReachedMax() bool           { return a.total >= a.Max }

// Response is a SOLR response.
type Response struct {
	Header struct {
		Status int `json:"status"`
		QTime  int `json:"QTime"`
		Params struct {
			Query      string `json:"q"`
			CursorMark string `json:"cursorMark"`
			Sort       string `json:"sort"`
			Rows       string `json:"rows"`
		} `json:"params"`
	} `json:"header"`
	Response struct {
		NumFound int               `json:"numFound"`
		Start    int               `json:"start"`
		Docs     []json.RawMessage `json:"docs"` // dependent on SOLR schema
	} `json:"response"`
	NextCursorMark string `json:"nextCursorMark"`
}
