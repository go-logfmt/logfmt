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
	s       *bufio.Scanner
	line    []byte
	key     []byte
	value   []byte
	lineNum int
	pos     int
	start   int
	err     error
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
	dec.pos = 0
	return true
}

func (dec *Decoder) ScanKeyval() bool {
	dec.key, dec.value = nil, nil
	if dec.err != nil || dec.isEol() {
		return false
	}

	// garbage
	for {
		c := dec.peek()
		switch {
		case c == '=' || c == '"':
			dec.unexpectedByte(c)
			return false
		case c > ' ':
			goto key
		}
		if !dec.skip() {
			return false
		}
	}

key:
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=':
			dec.key = dec.token(dec.pos)
			goto equal
		case c == '"':
			dec.unexpectedByte(c)
			return false
		case c <= ' ':
			dec.key = dec.token(dec.pos)
			return true
		}
		if !dec.skip() {
			dec.key = dec.token(dec.pos)
			return true
		}
	}

equal:
	ok := dec.skip()
	if !ok {
		return true
	}
	switch c := dec.peek(); {
	case c == '"':
		goto qvalue
	case c > ' ':
		goto ivalue
	}
	return true

ivalue:
	dec.start = dec.pos
	for {
		switch c := dec.peek(); {
		case c == '=' || c == '"':
			dec.unexpectedByte(c)
			return false
		case c <= ' ':
			dec.value = dec.token(dec.pos)
			return true
		}
		if !dec.skip() {
			dec.value = dec.token(dec.pos)
			return true
		}
	}

qvalue:
	dec.start = dec.pos
	for {
		if !dec.skip() {
			dec.syntaxError("unterminated quoted value")
			return false
		}
		c := dec.peek()
		switch {
		case c == '\\':
			goto qvalueEsc
		case c == '"':
			dec.start++
			dec.value = dec.token(dec.pos)
			dec.skip()
			return true
		}
	}

qvalueEsc:
	var esc bool
	for {
		c := dec.peek()
		switch {
		case esc:
			esc = false
		case c == '\\':
			esc = true
		case c == '"':
			dec.skip()
			v, ok := unquoteBytes(dec.token(dec.pos))
			if !ok {
				dec.syntaxError("invalid quoted value")
				return false
			}
			dec.value = v
			return true
		}
		if !dec.skip() {
			dec.syntaxError("unterminated quoted value")
			return false
		}
	}
}

func (dec *Decoder) Key() []byte {
	return dec.key
}

func (dec *Decoder) Value() []byte {
	return dec.value
}

func (dec *Decoder) Err() error {
	return dec.err
}

// func (dec *Decoder) DecodeValue() ([]byte, error) {
// }

func (dec *Decoder) peek() byte {
	return dec.line[dec.pos]
}

func (dec *Decoder) token(end int) []byte {
	if dec.start == end {
		return nil
	}
	return dec.line[dec.start:end]
}

func (dec *Decoder) isEol() bool {
	return dec.pos >= len(dec.line)
}

func (dec *Decoder) skip() bool {
	dec.pos++
	if dec.isEol() {
		return false
	}
	return true
}

func (dec *Decoder) syntaxError(msg string) {
	dec.err = &SyntaxError{
		Msg:  msg,
		Line: dec.lineNum,
		Pos:  dec.pos + 1,
	}
}

func (dec *Decoder) unexpectedByte(c byte) {
	dec.err = &SyntaxError{
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
