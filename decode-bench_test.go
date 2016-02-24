package logfmt

import (
	"bytes"
	"io"
	"testing"
)

func BenchmarkDecodeKeyval(b *testing.B) {
	const rows = 10000
	data := []byte{}
	for i := 0; i < rows; i++ {
		data = append(data, "a=1 b=\"bar\" ƒ=2h3s r=\"esc\t\" d x=sf   \n"...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var (
			dec = NewDecoder(bytes.NewReader(data))
			err error
		)
		j := 0
		for ; err == nil; j++ {
			_, _, err = dec.DecodeKeyval()
		}
		if err != io.EOF {
			b.Errorf("got %v, want %v", err, io.EOF)
		}
		if j < rows {
			b.Errorf("got %v, want %v", j, rows)
		}
	}
}
