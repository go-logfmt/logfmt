package logfmt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

var (
	ErrUnterminatedValue  = errors.New("unterminated quoted value")
	ErrInvalidQuotedValue = errors.New("invalid quoted value")
	EndOfRecord           = errors.New("end of record")
)

type Decoder struct {
	s          *bufio.Scanner
	line       []byte
	lineNum    int
	pos        int
	start, end int
	state      stateFn
	err        error
}

func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{
		s: bufio.NewScanner(r),
	}
	return dec
}

func (dec *Decoder) NextRecord() bool {
	if dec.err == EndOfRecord {
		dec.err = nil
	} else if dec.err != nil {
		return false
	}
	if !dec.s.Scan() {
		dec.err = dec.s.Err()
		return false
	}
	dec.lineNum++
	dec.line = dec.s.Bytes()
	if len(dec.line) > 0 {
		dec.state = garbage
	} else {
		dec.state = eol
	}
	dec.pos = 0
	return true
}

func (dec *Decoder) ScanKey() []byte {
	return dec.scanTok(tokKey)
}

func (dec *Decoder) ScanValue() []byte {
	return dec.scanTok(tokValue | tokQuotedValue)
}

func (dec *Decoder) Err() error {
	return dec.err
}

func (dec *Decoder) scanTok(toks tokType) []byte {
	var tt tokType
	for dec.err == nil && dec.state != nil && tt&toks == 0 {
		dec.state, tt, dec.err = dec.state(dec)
	}
	if tt&toks == 0 {
		return nil
	}
	if tt != tokQuotedValue {
		return dec.token()
	}
	t, ok := unquoteBytes(dec.token())
	if !ok {
		dec.err = ErrInvalidQuotedValue
		return nil
	}
	return t
}

// func (dec *Decoder) DecodeValue() ([]byte, error) {
// }

func (dec *Decoder) peek() byte {
	return dec.line[dec.pos]
}

func (dec *Decoder) token() []byte {
	if dec.start == dec.end {
		return nil
	}
	return dec.line[dec.start:dec.end]
}

func (dec *Decoder) skip() bool {
	dec.pos++
	if dec.pos >= len(dec.line) {
		return false
	}
	return true
}

type tokType int

const (
	tokNone tokType = 1 << iota
	tokKey
	tokEqual
	tokValue
	tokQuotedValue
	tokEOL
)

type stateFn func(*Decoder) (stateFn, tokType, error)

func garbage(dec *Decoder) (stateFn, tokType, error) {
	for {
		c := dec.peek()
		switch {
		case c == '=' || c == '"':
			return garbage, tokNone, dec.unexpectedByte(c)
		case c > ' ':
			return key, tokNone, nil
		}
		if !dec.skip() {
			return eol, tokNone, nil
		}
	}
}

func eol(dec *Decoder) (stateFn, tokType, error) {
	return eol, tokEOL, EndOfRecord
}

func key(dec *Decoder) (stateFn, tokType, error) {
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=':
			dec.end = dec.pos
			return equal, tokKey, nil
		case c == '"':
			return nil, tokNone, dec.unexpectedByte(c)
		case c <= ' ':
			dec.end = dec.pos
			return nvalue, tokKey, nil
		}
		if !dec.skip() {
			dec.end = dec.pos
			return eol, tokKey, nil
		}
	}
}

func equal(dec *Decoder) (stateFn, tokType, error) {
	dec.start = dec.pos
	ok := dec.skip()
	dec.end = dec.pos
	if !ok {
		return eol, tokEqual, nil
	}
	return value, tokEqual, nil
}

func value(dec *Decoder) (stateFn, tokType, error) {
	for {
		switch c := dec.peek(); {
		case c == '"':
			return qvalue, tokNone, nil
		case c > ' ':
			return ivalue, tokNone, nil
		}
		if !dec.skip() {
			dec.start = dec.pos
			dec.end = dec.pos
			return eol, tokValue, nil
		}
	}
}

func nvalue(dec *Decoder) (stateFn, tokType, error) {
	dec.start = dec.pos
	dec.end = dec.pos
	return garbage, tokValue, nil
}

func ivalue(dec *Decoder) (stateFn, tokType, error) {
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=' || c == '"':
			return nil, tokNone, dec.unexpectedByte(c)
		case c <= ' ':
			dec.end = dec.pos
			return garbage, tokValue, nil
		}
		if !dec.skip() {
			dec.end = dec.pos
			return eol, tokValue, nil
		}
	}
}

func qvalue(dec *Decoder) (stateFn, tokType, error) {
	dec.start = dec.pos
	for {
		if !dec.skip() {
			return eol, tokNone, ErrUnterminatedValue
		}
		c := dec.peek()
		switch {
		case c == '\\':
			return qvalueEsc, tokNone, nil
		case c == '"':
			dec.start++
			dec.end = dec.pos
			if !dec.skip() {
				return eol, tokValue, nil
			}
			return garbage, tokValue, nil
		}
	}
}

func qvalueEsc(dec *Decoder) (stateFn, tokType, error) {
	var esc bool
	for {
		c := dec.peek()
		switch {
		case esc:
			esc = false
		case c == '\\':
			esc = true
		case c == '"':
			ok := dec.skip()
			dec.end = dec.pos
			if !ok {
				return eol, tokQuotedValue, nil
			}
			return garbage, tokQuotedValue, nil
		}
		if !dec.skip() {
			return eol, tokNone, ErrUnterminatedValue
		}
	}
}

func (dec *Decoder) unexpectedByte(c byte) error {
	return &SyntaxError{
		Msg:  fmt.Sprintf("unexpected %q", c),
		Line: dec.lineNum,
		Pos:  dec.pos + 1,
	}
}

type SyntaxError struct {
	Msg  string
	Line int
	Pos  int
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("logfmt syntax error at pos %d on line %d: %s", e.Pos, e.Line, e.Msg)
}
