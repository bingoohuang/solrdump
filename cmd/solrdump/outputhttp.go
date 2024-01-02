package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/gg/pkg/rest"
	"github.com/bingoohuang/gg/pkg/vars"
)

func outputHttp(uri0 string, doc []byte, verbose int, printer Printer) {
	// 从doc中提取并替换uri中的变量
	// 例如uri为`127.0.0.1:9092/zz/docs?routing=@id`，则从doc（JSON格式)中取出key是id的值替换进去
	eval, err := vars.ParseExpr(uri0).Eval(&JsonValue{Doc: doc})
	if err != nil {
		log.Printf("ParseExpr %s error: %v", uri0, err)
		return
	}
	uri := eval.(string)
	if verbose >= 1 && uri != uri0 {
		printer.PutKey("request", fmt.Sprintf("http uri: %s", uri))
	}

	start := time.Now()
	r, err := restyClient.R().
		SetHeader("Content-Type", rest.ContentTypeJSON).
		SetBody(doc).Post(uri)
	cost := time.Since(start)
	if err != nil {
		log.Printf("sent to %s error %v", uri, err)
		return
	}

	statusCode := r.StatusCode
	if statusCode < 200 || statusCode >= 300 || verbose >= 2 {
		printer.PutKey("request body", string(doc))
	}

	if verbose >= 2 {
		body := r.Bytes()
		printer.PutKey("response", fmt.Sprintf("sent cost: %s status: %d, body: %s", cost, r.StatusCode, body))
	} else if verbose >= 1 {
		printer.PutKey("response", fmt.Sprintf("sent cost: %s status: %d", cost, r.StatusCode))
	}
}
