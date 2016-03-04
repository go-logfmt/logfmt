// +build gofuzz

package logfmt

import (
	"bytes"
	"fmt"
	"io"
)

type kv struct {
	k, v []byte
}

func parse(data []byte) ([][]kv, error) {
	var got [][]kv
	dec := NewDecoder(bytes.NewReader(data))
	for dec.NextRecord() {
		var kvs []kv
		for dec.Err() == nil {
			k := dec.ScanKey()
			v := dec.ScanValue()
			if k != nil {
				kvs = append(kvs, kv{k, v})
			}
		}
		if err := dec.Err(); err == EndOfRecord {
			got = append(got, kvs)
			kvs = nil
		}
	}
	return got, dec.Err()
}

func write(recs [][]kv, w io.Writer) error {
	enc := NewEncoder(w)
	for _, rec := range recs {
		for _, f := range rec {
			if err := enc.EncodeKeyval(f.k, f.v); err != nil {
				return err
			}
		}
		if err := enc.EndRecord(); err != nil {
			return err
		}
	}
	return nil
}

func Fuzz(data []byte) int {
	parsed, err := parse(data)
	if err != nil {
		return 0
	}
	var w1 bytes.Buffer
	if err := write(parsed, &w1); err != nil {
		panic(err)
	}
	parsed, err = parse(data)
	if err != nil {
		panic(err)
	}
	var w2 bytes.Buffer
	if err := write(parsed, &w2); err != nil {
		panic(err)
	}
	if !bytes.Equal(w1.Bytes(), w2.Bytes()) {
		panic(fmt.Sprintf("reserialized data does not match:\n%q\n%q\n", w1.Bytes(), w2.Bytes()))
	}
	return 1
}
