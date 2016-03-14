package logfmt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// EndOfRecord indicates that no more keys or values exist to decode in the
// current record. Use Decoder.ScanRecord to advance to the next record.
var EndOfRecord = errors.New("end of record")

// A Decoder reads and decodes logfmt records from an input stream.
type Decoder struct {
	s          *bufio.Scanner
	line       []byte
	key        []byte
	value      []byte
	lineNum    int
	pos        int
	start, end int
	state      stateFn
	err        error
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may read data from r beyond
// the logfmt records requested.
func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{
		s: bufio.NewScanner(r),
	}
	return dec
}

// ScanRecord advances the Decoder to the next record, which can then be
// parsed with the ScanKey and ScanValue methods. It returns false when
// decoding stops, either by reaching the end of the input or an error. After
// ScanRecord returns false, the Err method will return any error that
// occurred during decoding, except that if it was io.EOF, Err will return
// nil.
func (dec *Decoder) ScanRecord() bool {
	if dec.err != nil {
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

func (dec *Decoder) ScanKeyval() bool {
	dec.key = dec.scanKey()
	if dec.key == nil {
		return false
	}
	dec.value = dec.scanValue()
	return true
}

func (dec *Decoder) Key() []byte {
	return dec.key
}

func (dec *Decoder) Value() []byte {
	return dec.value
}

func (dec *Decoder) scanKey() []byte {
	var tt tokType
	for tt&(tokKey|tokEOL|tokErr) == 0 {
		tt = dec.state(dec)
	}
	if tt != tokKey {
		return nil
	}
	return dec.token()
}

func (dec *Decoder) scanValue() []byte {
	const toks = tokValue | tokQuotedValue
	var tt tokType
	for tt&(toks|tokEOL|tokErr) == 0 {
		tt = dec.state(dec)
	}
	if tt&toks == 0 {
		return nil
	}
	if tt != tokQuotedValue {
		return dec.token()
	}
	t, ok := unquoteBytes(dec.token())
	if !ok {
		dec.syntaxError("invalid quoted value")
		return nil
	}
	return t
}

func (dec *Decoder) Err() error {
	return dec.err
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
	tokErr
)

type stateFn func(*Decoder) tokType

func garbage(dec *Decoder) tokType {
	for {
		c := dec.peek()
		switch {
		case c == '=' || c == '"':
			return dec.unexpectedByte(c)
		case c > ' ':
			dec.state = key
			return tokNone
		}
		if !dec.skip() {
			dec.state = eol
			return tokNone
		}
	}
}

func err(dec *Decoder) tokType {
	return tokErr
}

func eol(dec *Decoder) tokType {
	return tokEOL
}

func key(dec *Decoder) tokType {
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=':
			dec.end = dec.pos
			dec.state = equal
			return tokKey
		case c == '"':
			return dec.unexpectedByte(c)
		case c <= ' ':
			dec.end = dec.pos
			dec.state = nvalue
			return tokKey
		}
		if !dec.skip() {
			dec.end = dec.pos
			dec.state = eol
			return tokKey
		}
	}
}

func equal(dec *Decoder) tokType {
	dec.start = dec.pos
	ok := dec.skip()
	dec.end = dec.pos
	if !ok {
		dec.state = eol
		return tokNone
	}

	switch c := dec.peek(); {
	case c == '"':
		dec.state = qvalue
		return tokNone
	case c > ' ':
		dec.state = ivalue
		return tokNone
	}
	dec.start = dec.pos
	dec.end = dec.pos
	dec.state = garbage
	return tokValue
}

func nvalue(dec *Decoder) tokType {
	dec.start = dec.pos
	dec.end = dec.pos
	dec.state = garbage
	return tokValue
}

func ivalue(dec *Decoder) tokType {
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=' || c == '"':
			return dec.unexpectedByte(c)
		case c <= ' ':
			dec.end = dec.pos
			dec.state = garbage
			return tokValue
		}
		if !dec.skip() {
			dec.end = dec.pos
			dec.state = eol
			return tokValue
		}
	}
}

func qvalue(dec *Decoder) tokType {
	dec.start = dec.pos
	for {
		if !dec.skip() {
			dec.syntaxError("unterminated quoted value")
			return tokNone
		}
		c := dec.peek()
		switch {
		case c == '\\':
			dec.state = qvalueEsc
			return tokNone
		case c == '"':
			dec.start++
			dec.end = dec.pos
			if !dec.skip() {
				dec.state = eol
				return tokValue
			}
			dec.state = garbage
			return tokValue
		}
	}
}

func qvalueEsc(dec *Decoder) tokType {
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
				dec.state = eol
				return tokQuotedValue
			}
			dec.state = garbage
			return tokQuotedValue
		}
		if !dec.skip() {
			return dec.syntaxError("unterminated quoted value")
		}
	}
}

func (dec *Decoder) syntaxError(msg string) tokType {
	dec.state = err
	dec.err = &SyntaxError{
		Msg:  msg,
		Line: dec.lineNum,
		Pos:  dec.pos + 1,
	}
	return tokErr
}

func (dec *Decoder) unexpectedByte(c byte) tokType {
	dec.state = err
	dec.err = &SyntaxError{
		Msg:  fmt.Sprintf("unexpected %q", c),
		Line: dec.lineNum,
		Pos:  dec.pos + 1,
	}
	return tokErr
}

type SyntaxError struct {
	Msg  string
	Line int
	Pos  int
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("logfmt syntax error at pos %d on line %d: %s", e.Pos, e.Line, e.Msg)
}
