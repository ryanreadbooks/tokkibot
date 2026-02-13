package frontmatter

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

var (
	ErrNoFrontmatter       = errors.New("no frontmatter found")
	ErrUnclosedFrontmatter = errors.New("frontmatter not properly closed")
)

var _ Marker = (*yamlMarker)(nil)

type (
	marshalerFunc func([]byte, any) error
	Marker        interface {
		starts() []byte
		ends() []byte
		marshal([]byte, any) error
	}
)

type yamlMarker struct{}

func (m *yamlMarker) starts() []byte {
	return []byte("---")
}

func (m *yamlMarker) ends() []byte {
	return []byte("---")
}

func (m *yamlMarker) marshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

type Parser struct {
	src *bufio.Reader
	dst *bytes.Buffer

	cursor int
	start  int
	end    int
}

func (r *Parser) readline() ([]byte, bool, error) {
	line, err := r.src.ReadBytes('\n')
	eof := errors.Is(err, io.EOF)
	if err != nil && !eof {
		return nil, false, err
	}

	r.cursor += len(line)
	if _, werr := r.dst.Write(line); werr != nil {
		return nil, eof, werr
	}

	return bytes.TrimSpace(line), eof, nil
}

func (r *Parser) parse(mrk Marker) ([]byte, error) {
	// First non-empty line must be the start marker
	foundStart := false
	for {
		line, eof, err := r.readline()
		if err != nil {
			return nil, err
		}

		// Skip empty lines only at the very beginning
		if len(line) == 0 {
			if eof {
				return nil, ErrNoFrontmatter
			}
			continue
		}

		// First non-empty line must be start marker
		if !bytes.Equal(line, mrk.starts()) {
			return nil, ErrNoFrontmatter
		}

		// Found start marker
		r.start = r.cursor
		foundStart = true
		break
	}

	if !foundStart {
		return nil, ErrNoFrontmatter
	}

	// Find the end marker
	for {
		lineStart := r.cursor
		line, eof, err := r.readline()
		if err != nil {
			return nil, err
		}

		if bytes.Equal(line, mrk.ends()) {
			r.end = lineStart
			break
		}

		if eof {
			return nil, ErrUnclosedFrontmatter
		}
	}

	// Extract result content (between start and end markers)
	result := r.dst.Bytes()[r.start:r.end]

	return result, nil
}

func (r *Parser) parseWithRest(mrk Marker) ([]byte, []byte, error) {
	result, err := r.parse(mrk)
	if err != nil {
		return nil, nil, err
	}

	// read the rest from src
	rest, err := io.ReadAll(r.src)
	if err != nil {
		return nil, nil, err
	}

	return result, rest, nil
}

func newParser(src *bufio.Reader, dst *bytes.Buffer) *Parser {
	return &Parser{
		src: src,
		dst: dst,
	}
}

// Parse parses the frontmatter from the source and marshals it to the destination.
func Parse(src []byte, dst any, marker Marker) error {
	parser := newParser(bufio.NewReader(bytes.NewReader(src)), bytes.NewBuffer([]byte{}))
	matter, err := parser.parse(marker)
	if err != nil {
		return err
	}

	return marker.marshal(matter, dst)
}

// ParseGet parses the frontmatter and returns the rest of the content of src.
func ParseGet(src []byte, dst any, marker Marker) ([]byte, error) {
	parser := newParser(bufio.NewReader(bytes.NewReader(src)), bytes.NewBuffer([]byte{}))
	matter, rest, err := parser.parseWithRest(marker)
	if err != nil {
		return nil, err
	}

	err = marker.marshal(matter, dst)
	if err != nil {
		return nil, err
	}

	return rest, nil
}

func ParseYaml(src []byte, dst any) error {
	return Parse(src, dst, &yamlMarker{})
}

func ParseGetYaml(src []byte, dst any) ([]byte, error) {
	return ParseGet(src, dst, &yamlMarker{})
}
