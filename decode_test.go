package logfmt

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type kv struct {
	k, v []byte
}

func (s kv) String() string {
	return fmt.Sprintf("{k:%q v:%q}", s.k, s.v)
}

func TestDecoder_scan(t *testing.T) {
	tests := []struct {
		data string
		want [][]kv
	}{
		{"", nil},
		{"\n\n", [][]kv{nil, nil}},
		{
			"a=1\n",
			[][]kv{{{[]byte("a"), []byte("1")}}},
		},
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
		{"y=\"\\tf\"", [][]kv{{{[]byte("y"), []byte("\tf")}}}},
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
			"y= d y=g",
			[][]kv{{
				{[]byte("y"), nil},
				{[]byte("d"), nil},
				{[]byte("y"), []byte("g")},
			}},
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
			t.Errorf("\n  in: %q\n got: %+v\nwant: %+v", test.data, got, test.want)
		}
	}
}

func TestDecoder_errors(t *testing.T) {
	tests := []struct {
		data string
		want error
	}{
		{
			"a=1\n=bar",
			&SyntaxError{
				Msg:  "unexpected '='",
				Line: 2,
				Pos:  1,
			},
		},
		{
			"a=1\n\"k\"=bar",
			&SyntaxError{
				Msg:  "unexpected '\"'",
				Line: 2,
				Pos:  1,
			},
		},
		{
			"a=1\nk\"ey=bar",
			&SyntaxError{
				Msg:  "unexpected '\"'",
				Line: 2,
				Pos:  2,
			},
		},
		{
			"a=1\nk=b\"ar",
			&SyntaxError{
				Msg:  "unexpected '\"'",
				Line: 2,
				Pos:  4,
			},
		},
		{
			"a=1\nk=b=ar",
			&SyntaxError{
				Msg:  "unexpected '='",
				Line: 2,
				Pos:  4,
			},
		},
		{
			"a=\"1",
			&SyntaxError{
				Msg:  "unterminated quoted value",
				Line: 1,
				Pos:  5,
			},
		},
		{
			"a=\"\\t1",
			&SyntaxError{
				Msg:  "unterminated quoted value",
				Line: 1,
				Pos:  7,
			},
		},
		{
			"a=\"\\u1\"",
			&SyntaxError{
				Msg:  "invalid quoted value",
				Line: 1,
				Pos:  8,
			},
		},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.data))

		for dec.NextRecord() {
			for dec.Err() == nil {
				dec.ScanKey()
				dec.ScanValue()
			}
		}
		if got, want := dec.Err(), test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("got: %v, want: %v", got, want)
		}
	}
}

func TestDecoder_decode_encode(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"", ""},
		{"\n", "\n"},
		{"\n  \n", "\n\n"},
		{
			"a=1\nb=2\n",
			"a=1\nb=2\n",
		},
		{
			"a=1 b=\"bar\" ƒ=2h3s r=\"esc\\t\" d x=sf   ",
			"a=1 b=bar ƒ=2h3s r=\"esc\\t\" d= x=sf\n",
		},
	}

	for _, test := range tests {
		dec := NewDecoder(strings.NewReader(test.in))
		buf := bytes.Buffer{}
		enc := NewEncoder(&buf)

		var err error
	loop:
		for dec.NextRecord() && err == nil {
			for dec.Err() == nil {
				k := dec.ScanKey()
				v := dec.ScanValue()
				if k == nil {
					continue
				}
				if err = enc.EncodeKeyval(k, v); err != nil {
					break loop
				}
			}
			if err := dec.Err(); err == EndOfRecord {
				enc.EndRecord()
			}
		}
		if err == nil {
			err = dec.Err()
		}
		if err != nil {
			t.Errorf("got err: %v", err)
		}
		if got, want := buf.String(), test.out; got != want {
			t.Errorf("\n got: %q\nwant: %q", got, want)
		}
	}
}
