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
	pos     int
	key     []byte
	value   []byte
	lineNum int
	s       *bufio.Scanner
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
	dec.pos = 0
	return true
}

func (dec *Decoder) ScanKeyval() bool {
	dec.key, dec.value = nil, nil
	if dec.err != nil {
		return false
	}

	line := dec.s.Bytes()
	if dec.pos >= len(line) {
		return false
	}

	// garbage
	for line[dec.pos] <= ' ' {
		dec.pos++
		if dec.pos >= len(line) {
			return false
		}
	}

	start := dec.pos
	// key
	for {
		switch c := line[dec.pos]; {
		case c == '=':
			if dec.pos > start {
				dec.key = line[start:dec.pos]
			}
			if dec.key == nil {
				dec.unexpectedByte(c)
				return false
			}
			goto equal
		case c == '"':
			dec.unexpectedByte(c)
			return false
		case c <= ' ':
			if dec.pos > start {
				dec.key = line[start:dec.pos]
			}
			return true
		}
		dec.pos++
		if dec.pos >= len(line) {
			if dec.pos > start {
				dec.key = line[start:dec.pos]
			}
			return true
		}
	}

equal:
	dec.pos++
	if dec.pos >= len(line) {
		return true
	}
	switch c := line[dec.pos]; {
	case c <= ' ':
		return true
	case c == '"':
		goto qvalue
	}

	// value
	start = dec.pos
	for {
		switch c := line[dec.pos]; {
		case c == '=' || c == '"':
			dec.unexpectedByte(c)
			return false
		case c <= ' ':
			if dec.pos > start {
				dec.value = line[start:dec.pos]
			}
			return true
		}
		dec.pos++
		if dec.pos >= len(line) {
			if dec.pos > start {
				dec.value = line[start:dec.pos]
			}
			return true
		}
	}

qvalue:
	const (
		untermQuote  = "unterminated quoted value"
		invalidQuote = "invalid quoted value"
	)

	hasEsc := false
	start = dec.pos
	for {
		dec.pos++
		if dec.pos >= len(line) {
			dec.syntaxError(untermQuote)
			return false
		}
		switch line[dec.pos] {
		case '\\':
			hasEsc = true
			dec.pos++
			if dec.pos >= len(line) {
				dec.syntaxError(untermQuote)
				return false
			}
		case '"':
			if hasEsc {
				dec.pos++
				v, ok := unquoteBytes(line[start:dec.pos])
				if !ok {
					dec.syntaxError(invalidQuote)
					return false
				}
				dec.value = v
			} else {
				start++
				if dec.pos > start {
					dec.value = line[start:dec.pos]
				}
				dec.pos++
			}
			return true
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
