package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
)

func (App) VersionInfo() string { return "0.1.7 2021-06-09 13:51:38" }

func (a App) Usage() string {
	return fmt.Sprintf(`
Usage of %s (%s):
  -max int       Max number of rows (default 10)
  -q string      SOLR query (default "*:*")
  -rows int      Number of rows returned per request (default 10000)
  -bulk int      Number of rows in an elasticseach bulk (default 100)
  -server string SOLR server with index name, eg. localhost:8983/solr/example
  -version       Show version and exit
  -remove-fields Remove fields, _version_ defaulted
  -output        Output file, or http url, or noop
  -cursor        Enable cursor or not
  -v             Verbose, -vv -vvv
`, os.Args[0], a.VersionInfo())
}

type App struct {
	Server       string `required:"true"`
	Q            string `val:"*:*"`
	Max          int    `val:"10"`
	Rows         int    `val:"10000"`
	Bulk         int    `val:"100"`
	Version      bool
	Cursor       bool `val:"true"`
	RemoveFields []string
	Output       []string
	Verbose      int `flag:"v" count:"true"`

	baseURL  string
	query    url.Values
	total    int
	outputFn func(doc []byte)
	Context  context.Context
	closers  []io.Closer

	printer    Printer
	ResponseCh chan Response

	wg *sync.WaitGroup
}
