package logfmt

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestDecoder_token(t *testing.T) {
	tests := []struct {
		data string
		want []Token
	}{
		{
			`a=1 b="bar" ƒ=2h3s r="esc\t" d x=sf   `,
			[]Token{
				Key([]byte("a")), Value([]byte("1")),
				Key([]byte("b")), Value([]byte("bar")),
				Key([]byte("ƒ")), Value([]byte("2h3s")),
				Key([]byte("r")), Value([]byte("esc\t")),
				Key([]byte("d")), Value(nil),
				Key([]byte("x")), Value([]byte("sf")),
			},
		},
		{`x= `, []Token{Key([]byte("x")), Value(nil)}},
		{`y=`, []Token{Key([]byte("y")), Value(nil)}},
		{`y`, []Token{Key([]byte("y")), Value(nil)}},
		{`y=f`, []Token{Key([]byte("y")), Value([]byte("f"))}},
		{
			"y=f\ny=g",
			[]Token{
				Key([]byte("y")), Value([]byte("f")),
				EndOfRecord{},
				Key([]byte("y")), Value([]byte("g")),
			},
		},
		{
			"y=f  \n\x1e y=g",
			[]Token{
				Key([]byte("y")), Value([]byte("f")),
				EndOfRecord{},
				Key([]byte("y")), Value([]byte("g")),
			},
		},
		{
			"y=\"f\"\ny=g",
			[]Token{
				Key([]byte("y")), Value([]byte("f")),
				EndOfRecord{},
				Key([]byte("y")), Value([]byte("g")),
			},
		},
		{
			"y=\"f\n\"y=g",
			[]Token{
				Key([]byte("y")), Value([]byte("f\n")),
				Key([]byte("y")), Value([]byte("g")),
			},
		},
	}

	for _, test := range tests {
		var got []Token
		dec := NewDecoder(strings.NewReader(test.data))
		token, err := dec.Token()
		for ; token != nil; token, err = dec.Token() {
			got = append(got, token)
		}
		if err != io.EOF {
			t.Errorf("got err: %v, want: %v", err, io.EOF)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("\n got: %#v\nwant: %#v", got, test.want)
		}
	}
}

func TestDecoder_decodeKeyval(t *testing.T) {
	tests := []struct {
		data string
		want [][2][]byte
	}{
		{
			`a=1 b="bar" ƒ=2h3s r="esc\t" d x=sf   `,
			[][2][]byte{
				{[]byte("a"), []byte("1")},
				{[]byte("b"), []byte("bar")},
				{[]byte("ƒ"), []byte("2h3s")},
				{[]byte("r"), []byte("esc\t")},
				{[]byte("d"), nil},
				{[]byte("x"), []byte("sf")},
			},
		},
	}

	for _, test := range tests {
		var got [][2][]byte
		dec := NewDecoder(strings.NewReader(test.data))
		k, v, err := dec.DecodeKeyval()
		for ; err == nil; k, v, err = dec.DecodeKeyval() {
			got = append(got, [2][]byte{k, v})
		}
		if err != io.EOF {
			t.Errorf("got err: %v, want: %v", err, io.EOF)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("\n got: %#v\nwant: %#v", got, test.want)
		}
	}
}
