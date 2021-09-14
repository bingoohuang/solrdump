package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"

	"github.com/bingoohuang/gg/pkg/v"
)

func (Arg) VersionInfo() string { return v.Version() }

func (a Arg) Usage() string {
	return fmt.Sprintf(`
Usage of %s:
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
  -routing       Routing keyword, default "routing", maybe "_routing"
`, os.Args[0])
}

type Arg struct {
	Config  string `flag:"c" usage:"yml config filepath"`
	Init    bool
	Version bool

	Routing      string `val:"routing"`
	Server       string `required:"true"`
	Q            string `val:"*:*"`
	Max          int    `val:"10"`
	Rows         int    `val:"10000"`
	Bulk         int    `val:"100"`
	Cursor       bool   `val:"true"`
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

	outputWg *sync.WaitGroup
}
