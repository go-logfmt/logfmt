package logfmt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

type Decoder struct {
	r     *bufio.Reader
	state stateFn
	err   error
}

func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{
		r: bufio.NewReader(r),
	}
	dec.state = garbage
	return dec
}

func (dec *Decoder) DecodeKeyval() (key, value []byte, err error) {
	k, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if _, ok := k.(EndOfRecord); ok {
		return nil, nil, nil
	}
	kk, ok := k.(Key)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected token, wanted %T, got %T", kk, k)
	}
	v, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	vv, ok := v.(Value)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected token, wanted %T, got %T", vv, v)
	}
	return kk, vv, nil
}

// func (dec *Decoder) NextRecord() error {
// }

func (dec *Decoder) Token() (Token, error) {
	var t Token
	for dec.err == nil && dec.state != nil && t == nil {
		dec.state, t, dec.err = dec.state(dec)
	}
	return t, dec.err
}

type stateFn func(*Decoder) (stateFn, Token, error)

func garbage(dec *Decoder) (stateFn, Token, error) {
	for {
		c, err := dec.r.ReadByte()
		if err != nil {
			return nil, nil, err
		}
		switch {
		case c == '\n':
			return garbage, EndOfRecord{}, nil
		case c > ' ' && c != '"' && c != '=':
			return key, nil, dec.r.UnreadByte()
		}
	}
}

func key(dec *Decoder) (stateFn, Token, error) {
	var k []byte
	for {
		c, err := dec.r.ReadByte()
		if err == io.EOF {
			return nvalue, Key(k), nil
		}
		if err != nil {
			return nil, Key(k), err
		}
		switch {
		case c > ' ' && c != '"' && c != '=':
			k = append(k, c)
		case c == '=':
			return equal, Key(k), nil
		default:
			return nvalue, Key(k), dec.r.UnreadByte()
		}
	}
}

func equal(dec *Decoder) (stateFn, Token, error) {
	for {
		c, err := dec.r.ReadByte()
		if err == io.EOF {
			return nvalue, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
		switch {
		case c > ' ' && c != '"' && c != '=':
			return ivalue, nil, dec.r.UnreadByte()
		case c == '"':
			return qvalue, nil, dec.r.UnreadByte()
		default:
			return nvalue, nil, dec.r.UnreadByte()
		}
	}
}

func nvalue(dec *Decoder) (stateFn, Token, error) {
	return garbage, Value(nil), nil
}

func ivalue(dec *Decoder) (stateFn, Token, error) {
	var v []byte
	for {
		c, err := dec.r.ReadByte()
		if err != nil {
			return nil, Value(v), err
		}
		switch {
		case c > ' ' && c != '"' && c != '=':
			v = append(v, c)
		default:
			return garbage, Value(v), dec.r.UnreadByte()
		}
	}
}

var (
	ErrUnterminatedValue  = errors.New("unterminated quoted value")
	ErrInvalidQuotedValue = errors.New("invalid quoted value")
)

func qvalue(dec *Decoder) (stateFn, Token, error) {
	var v []byte
	hasEsc, esc := false, false
	for {
		c, err := dec.r.ReadByte()
		if err != nil {
			return nil, nil, ErrUnterminatedValue
		}
		switch {
		case esc:
			v = append(v, c)
			esc = false
		case c == '\\':
			v = append(v, c)
			hasEsc, esc = true, true
		case c == '"':
			v = append(v, c)
			if len(v) == 1 {
				break
			}
			if !hasEsc {
				return garbage, Value(v[1 : len(v)-1]), nil
			}
			uq, ok := unquoteBytes(v)
			if !ok {
				return nil, nil, ErrInvalidQuotedValue
			}
			return garbage, Value(uq), nil
		default:
			v = append(v, c)
		}
	}
}

// Token holds a value of one of these types:
//
//    Key
//    Value
//    EndOfRecord
type Token interface{}

type Key []byte

type Value []byte

type EndOfRecord struct{}
