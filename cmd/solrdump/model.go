package main

import (
	"fmt"
	"io"
	"log"

	"github.com/bingoohuang/jj"
)

type JsonValue struct {
	Doc []byte
}

func (j *JsonValue) Value(name, _ string) interface{} { return jj.GetBytes(j.Doc, name).String() }

type Printer interface {
	io.Closer
	Put(v interface{})
	PutKey(k string, v interface{})
}

type LogPrinter struct{}

func (l LogPrinter) Close() error                   { return nil }
func (l LogPrinter) Put(v interface{})              { log.Print(v) }
func (l LogPrinter) PutKey(k string, v interface{}) { log.Print(fmt.Sprintf("%s: %v", k, v)) }

type closeFn func()

func (f closeFn) Close() error { f(); return nil }
