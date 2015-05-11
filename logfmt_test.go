package logfmt_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logfmt/logfmt"
)

func TestEncodeKeyValue(t *testing.T) {
	nilPtr := (*int)(nil)

	data := []struct {
		key, value interface{}
		want       string
		err        error
	}{
		{key: nil, value: nil, err: logfmt.ErrNilKey},
		{key: nilPtr, value: nil, err: logfmt.ErrNilKey},
		{key: "k", value: nil, want: "k=nil"},
		{key: " ", value: "v", err: logfmt.ErrInvalidKey},
		{key: "=", value: "v", err: logfmt.ErrInvalidKey},
		{key: `"`, value: "v", err: logfmt.ErrInvalidKey},
		{key: stringMarshaler(" "), value: "v", err: logfmt.ErrInvalidKey},
		{key: stringData(" "), value: "v", err: logfmt.ErrInvalidKey},
		{key: `\`, value: "v", want: `\=v`},
		{key: "k", value: nilPtr, want: "k=nil"},
		{key: "k", value: "", want: "k="},
		{key: "k", value: "nil", want: `k="nil"`},
		{key: "k", value: "<nil>", want: `k=<nil>`},
		{key: "k", value: "v", want: "k=v"},
		{key: "k", value: true, want: "k=true"},
		{key: "k", value: 1, want: "k=1"},
		{key: "k", value: 1.025, want: "k=1.025"},
		{key: "k", value: 1e-3, want: "k=0.001"},
		{key: "k", value: 3.5 + 2i, want: "k=(3.5+2i)"},
		{key: "k", value: "v v", want: `k="v v"`},
		{key: "k", value: " ", want: `k=" "`},
		{key: "k", value: `"`, want: `k="\""`},
		{key: "k", value: `=`, want: `k="="`},
		{key: "k", value: `\`, want: `k=\`},
		{key: "k", value: `=\`, want: `k="=\\"`},
		{key: "k", value: `\"`, want: `k="\\\""`},
		{key: "k", value: [2]int{2, 19}, want: `k[0]=2 k[1]=19"`},
		{key: "k", value: []string{"e1", "e 2"}, want: `k[0]=e1 k[1]="e 2"`},
		{key: "k", value: structData{"a a", 9}, want: `k.fieldA="a a" k.B=9`},
		{key: "k", value: decimalMarshaler{5, 9}, want: "k=5.9"},
		{key: "k", value: (*decimalMarshaler)(nil), want: "k=nil"},
		{key: "k", value: decimalStringer{5, 9}, want: "k=5.9"},
		{key: "k", value: (*decimalStringer)(nil), want: "k=nil"},
		{key: "k", value: marshalerStringer{5, 9}, want: "k=5.9"},
		{key: "k", value: (*marshalerStringer)(nil), want: "k=nil"},
		{key: (*marshalerStringer)(nil), value: "v", err: logfmt.ErrNilKey},
		{key: decimalMarshaler{5, 9}, value: "v", want: "5.9=v"},
		{key: (*decimalMarshaler)(nil), value: "v", err: logfmt.ErrNilKey},
		{key: decimalStringer{5, 9}, value: "v", want: "5.9=v"},
		{key: (*decimalStringer)(nil), value: "v", err: logfmt.ErrNilKey},
		{key: marshalerStringer{5, 9}, value: "v", want: "5.9=v"},
	}

	for _, d := range data {
		w := &bytes.Buffer{}
		enc := logfmt.NewEncoder(w)
		err := enc.EncodeKeyValue(d.key, d.value)
		if err != d.err {
			t.Errorf("%#v, %#v: got error: %v, want error: %v", d.key, d.value, err, d.err)
		}
		if got, want := w.String(), d.want; got != want {
			t.Errorf("%#v, %#v: got '%s', want '%s'", d.key, d.value, got, want)
		}
	}
}

func TestMarshalKeyvals(t *testing.T) {
	one := 1
	ptr := &one
	nilPtr := (*int)(nil)

	data := []struct {
		in   []interface{}
		want []byte
		err  error
	}{
		{in: nil, want: nil},
		{in: kv(), want: nil},
		{in: kv(nil, "v"), err: logfmt.ErrNilKey},
		{in: kv(nilPtr, "v"), err: logfmt.ErrNilKey},
		{in: kv("k"), want: []byte("k=nil")},
		{in: kv("k", nil), want: []byte("k=nil")},
		{in: kv("k", ""), want: []byte("k=")},
		{in: kv("k", "nil"), want: []byte(`k="nil"`)},
		{in: kv("k", "<nil>"), want: []byte(`k=<nil>`)},
		{in: kv("k", "v"), want: []byte("k=v")},
		{in: kv("k", true), want: []byte("k=true")},
		{in: kv("k", 1), want: []byte("k=1")},
		{in: kv("k", ptr), want: []byte("k=1")},
		{in: kv("k", nilPtr), want: []byte("k=nil")},
		{in: kv("k", 1.025), want: []byte("k=1.025")},
		{in: kv("k", 1e-3), want: []byte("k=0.001")},
		{in: kv("k", "v v"), want: []byte(`k="v v"`)},
		{in: kv("k", `"`), want: []byte(`k="\""`)},
		{in: kv("k", `=`), want: []byte(`k="="`)},
		{in: kv("k", `\`), want: []byte(`k=\`)},
		{in: kv("k", `=\`), want: []byte(`k="=\\"`)},
		{in: kv("k", `\"`), want: []byte(`k="\\\""`)},
		{in: kv("k1", "v1", "k2", "v2"), want: []byte("k1=v1 k2=v2")},
		{in: kv("k", decimalMarshaler{5, 9}), want: []byte("k=5.9")},
		{in: kv("k", (*decimalMarshaler)(nil)), want: []byte("k=nil")},
		{in: kv("k", decimalStringer{5, 9}), want: []byte("k=5.9")},
		{in: kv("k", (*decimalStringer)(nil)), want: []byte("k=nil")},
		{in: kv("k", marshalerStringer{5, 9}), want: []byte("k=5.9")},
		{in: kv("k", (*marshalerStringer)(nil)), want: []byte("k=nil")},
		{in: kv(one, "v"), want: []byte("1=v")},
		{in: kv(ptr, "v"), want: []byte("1=v")},
		{in: kv((*marshalerStringer)(nil), "v"), err: logfmt.ErrNilKey},
		{in: kv(decimalMarshaler{5, 9}, "v"), want: []byte("5.9=v")},
		{in: kv((*decimalMarshaler)(nil), "v"), err: logfmt.ErrNilKey},
		{in: kv(decimalStringer{5, 9}, "v"), want: []byte("5.9=v")},
		{in: kv((*decimalStringer)(nil), "v"), err: logfmt.ErrNilKey},
		{in: kv(marshalerStringer{5, 9}, "v"), want: []byte("5.9=v")},
	}

	for _, d := range data {
		got, err := logfmt.MarshalKeyvals(d.in...)
		if err != d.err {
			t.Errorf("unexpected error marshaling %+v: %v", d.in, err)
			continue
		}
		if got, want := got, d.want; !reflect.DeepEqual(got, want) {
			t.Errorf("%#v: got '%s', want '%s'", d.in, got, want)
		}
	}
}

func kv(keyvals ...interface{}) []interface{} {
	return keyvals
}

type stringData string

type structData struct {
	A string `logfmt:"fieldA"`
	B int
}

type stringMarshaler string

func (s stringMarshaler) MarshalText() ([]byte, error) {
	return []byte(s), nil
}

type decimalMarshaler struct {
	a, b int
}

func (t decimalMarshaler) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "%d.%d", t.a, t.b)
	return buf.Bytes(), nil
}

type decimalStringer struct {
	a, b int
}

func (s decimalStringer) String() string {
	return fmt.Sprintf("%d.%d", s.a, s.b)
}

type marshalerStringer struct {
	a, b int
}

func (t marshalerStringer) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "%d.%d", t.a, t.b)
	return buf.Bytes(), nil
}

func (t marshalerStringer) String() string {
	return fmt.Sprint(t.a + t.b)
}
