package logfmt

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecoder_token(t *testing.T) {
	type kv struct {
		k, v []byte
	}

	tests := []struct {
		data string
		want [][]kv
	}{
		{
			`a=1 b="bar" ƒ=2h3s r="esc\t" d x=sf   `,
			[][]kv{{
				{[]byte("a"), []byte("1")},
				{[]byte("b"), []byte("bar")},
				{[]byte("ƒ"), []byte("2h3s")},
				{[]byte("r"), []byte("esc\t")},
				{[]byte("d"), nil},
				{[]byte("x"), []byte("sf")},
			}},
		},
		{`x= `, [][]kv{{{[]byte("x"), nil}}}},
		{`y=`, [][]kv{{{[]byte("y"), nil}}}},
		{`y`, [][]kv{{{[]byte("y"), nil}}}},
		{`y=f`, [][]kv{{{[]byte("y"), []byte("f")}}}},
		{
			"y=f\ny=g",
			[][]kv{
				{{[]byte("y"), []byte("f")}},
				{{[]byte("y"), []byte("g")}},
			},
		},
		{
			"y=f  \n\x1e y=g",
			[][]kv{
				{{[]byte("y"), []byte("f")}},
				{{[]byte("y"), []byte("g")}},
			},
		},
		{
			"y=\"f\"\ny=g",
			[][]kv{
				{{[]byte("y"), []byte("f")}},
				{{[]byte("y"), []byte("g")}},
			},
		},
		{
			"y=\"f\\n\"y=g",
			[][]kv{{
				{[]byte("y"), []byte("f\n")},
				{[]byte("y"), []byte("g")},
			}},
		},
	}

	for _, test := range tests {
		var got [][]kv
		dec := NewDecoder(strings.NewReader(test.data))

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
		if err := dec.Err(); err != nil {
			t.Errorf("got err: %v", err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("\n got: %#v\nwant: %#v", got, test.want)
		}
	}
}
