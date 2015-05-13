package logfmt

import (
	"bytes"
	"fmt"
	"testing"
)

func TestSafeString(t *testing.T) {
	_, ok := safeString((*stringer)(nil))
	if got, want := ok, false; got != want {
		t.Errorf(" got %v, want %v", got, want)
	}
}

func TestSafeMarshal(t *testing.T) {
	kb, err := safeMarshal((*stringMarshaler)(nil))
	if got := kb; got != nil {
		t.Errorf(" got %v, want nil", got)
	}
	if got, want := err, error(nil); got != want {
		t.Errorf(" got %v, want %v", got, want)
	}
}

type stringer int

func (s stringer) String() string {
	return fmt.Sprintf("%d", s)
}

func TestWriteKeyStrings(t *testing.T) {
	keygen := []func(string) interface{}{
		func(s string) interface{} { return s },
		func(s string) interface{} { return stringData(s) },
		func(s string) interface{} { return stringStringer(s) },
		func(s string) interface{} { return stringMarshaler(s) },
	}

	data := []struct {
		key  string
		want string
		err  error
	}{
		{key: "k", want: "k"},
		{key: `\`, want: `\`},
		{key: "\n", err: ErrInvalidKey},
		{key: "\x00", err: ErrInvalidKey},
		{key: "\x10", err: ErrInvalidKey},
		{key: "\x1F", err: ErrInvalidKey},
		{key: "", err: ErrInvalidKey},
		{key: " ", err: ErrInvalidKey},
		{key: "=", err: ErrInvalidKey},
		{key: `"`, err: ErrInvalidKey},
	}

	for _, g := range keygen {
		for _, d := range data {
			w := &bytes.Buffer{}
			enc := NewEncoder(w)
			key := g(d.key)
			err := enc.writeKey(key)
			if err != d.err {
				t.Errorf("%#v (%[1]T): got error: %v, want error: %v", key, err, d.err)
			}
			if err != nil {
				continue
			}
			if got, want := w.String(), d.want; got != want {
				t.Errorf("%#v (%[1]T): got '%s', want '%s'", key, got, want)
			}
		}
	}
}

func TestWriteKey(t *testing.T) {
	var (
		nilPtr *int
	)

	data := []struct {
		key  interface{}
		want string
		err  error
	}{
		{key: nil, err: ErrNilKey},
		{key: nilPtr, err: ErrNilKey},
		{key: (*stringStringer)(nil), err: ErrNilKey},
		{key: (*stringMarshaler)(nil), err: ErrNilKey},
		{key: (*stringerMarshaler)(nil), err: ErrNilKey},

		{key: make(chan int), err: ErrUnsportedType},
		{key: []int{}, err: ErrUnsportedType},
		{key: map[int]int{}, err: ErrUnsportedType},
		{key: [2]int{}, err: ErrUnsportedType},
		{key: struct{}{}, err: ErrUnsportedType},
		{key: fmt.Sprint, err: ErrUnsportedType},
	}

	for _, d := range data {
		w := &bytes.Buffer{}
		enc := NewEncoder(w)
		err := enc.writeKey(d.key)
		if err != d.err {
			t.Errorf("%#v: got error: %v, want error: %v", d.key, err, d.err)
		}
		if err != nil {
			continue
		}
		if got, want := w.String(), d.want; got != want {
			t.Errorf("%#v: got '%s', want '%s'", d.key, got, want)
		}
	}
}

type stringData string

type stringStringer string

func (s stringStringer) String() string {
	return string(s)
}

type stringMarshaler string

func (s stringMarshaler) MarshalText() ([]byte, error) {
	return []byte(s), nil
}

type stringerMarshaler string

func (s stringerMarshaler) String() string {
	return string(s)
}

func (s stringerMarshaler) MarshalText() ([]byte, error) {
	return []byte(s), nil
}
