package main

import (
	"bytes"
	"fmt"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/jj"
	"github.com/gobars/solrdump/badger"
	"github.com/gobars/solrdump/pester"
	"net/url"
	"path"
	"strings"
	"time"
)

type Arg struct {
	Es     string `val:"192.168.126.5:9202"`
	Index  string `val:"license"`
	Type   string `val:"docs"`
	Batch  int    `val:"10000"`
	Max    int64  `val:"100000"`
	Filter string `val:"hits.hits.#._source.holderIdentityNum.0"`
	Badger string `val:"es-badger-db"`
}

func main() {
	arg := &Arg{}
	flagparse.Parse(arg)

	uri, _ := UrlJoin(arg.Es, map[string]string{"scroll": "1m"}, arg.Index, arg.Type, `/_search`)

	//uri := `http://192.168.126.5:9202/license/docs/_search?scroll=1m`
	query := fmt.Sprintf(`{"size":%d,"_source":["holderIdentityNum"]}`, arg.Batch)

	start := time.Now()
	r, err := pester.Post(uri, rest.ContentTypeJSON, strings.NewReader(query))
	cost := time.Since(start)
	if err != nil {
		panic(err)
	}

	scrollUri, _ := UrlJoin(arg.Es, nil, "/_search/scroll")
	payloadTemplate := []byte(`{"scroll_id":"","scroll":"1m"}`)
	totalHits := int64(0)

	db, _ := badger.Open(arg.Badger)
	defer db.Close()
	index := uint64(0)

	for {
		body, _ := rest.ReadCloseBody(r)
		hits := jj.GetBytes(body, "hits.hits.#").Int()
		if hits <= 0 || (arg.Max > 0 && totalHits >= arg.Max) {
			break
		}

		totalHits += hits

		result := jj.GetBytes(body, arg.Filter)
		result.ForEach(func(_, c jj.Result) bool {
			db.Set(badger.Uint64ToBytes(index), []byte(c.String()))
			index++
			return true
		})

		scrollID := jj.GetBytes(body, "_scroll_id")
		payload, _ := jj.SetBytes(payloadTemplate, "scroll_id", scrollID.String())

		start = time.Now()
		r, err = pester.Post(scrollUri, rest.ContentTypeJSON, bytes.NewReader(payload))
		if err != nil {
			panic(err)
		}
		cost += time.Since(start)
	}

	fmt.Printf("total hists %d, cost %s\n", totalHits, cost)
}

func UrlJoin(basePath string, query map[string]string, paths ...string) (string, error) {
	basePath, err := rest.FixURI(basePath)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid url")
	}

	p2 := append([]string{u.Path}, paths...)
	u.Path = path.Join(p2...)

	if query != nil {
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}
