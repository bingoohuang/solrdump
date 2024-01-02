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

func (j *JsonValue) Value(name, _, _ string) (any, error) {
	return jj.GetBytes(j.Doc, name).String(), nil
}

type Printer interface {
	io.Closer
	Put(v any)
	PutKey(k string, v any)
}

type LogPrinter struct{}

func (l LogPrinter) Close() error           { return nil }
func (l LogPrinter) Put(v any)              { log.Print(v) }
func (l LogPrinter) PutKey(k string, v any) { log.Print(fmt.Sprintf("%s: %v", k, v)) }

type closeFn func()

func (f closeFn) Close() error { f(); return nil }
