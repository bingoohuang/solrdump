package badger

import (
	"encoding/binary"
	"errors"
	"github.com/dgraph-io/badger/v3"
)

type Badger struct {
	DB *badger.DB
}

func (b *Badger) Close() error { return b.DB.Close() }

func (b *Badger) Get(k []byte) (val []byte, er error) {
	er = b.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}

		val, err = item.ValueCopy(nil)
		return err
	})

	return
}

func (b *Badger) Set(k, v []byte) error {
	e := badger.NewEntry(k, v)
	return b.DB.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(e)
	})
}

func Open(path string) (*Badger, error) {
	options := badger.DefaultOptions(path)
	options.Logger = nil
	db, err := badger.Open(options)
	if err != nil {
		return nil, err
	}

	return &Badger{DB: db}, nil
}

func Uint64ToBytes(i uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], i)
	return buf[:]
}

func BytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
