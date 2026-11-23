package billxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// maxDepth bounds element nesting so hostile input cannot exhaust the stack.
const maxDepth = 200

var (
	errTooDeep    = errors.New("xml: nesting too deep")
	errUnfinished = errors.New("xml: unexpected end of document")
)

// node is one XML element with its direct content in document order.
type node struct {
	name  string
	attrs map[string]string
	kids  []*node // child elements and text pseudo-nodes (name == "")
	text  string  // character data, set only when name == ""
	start int     // byte offset of the start tag in the source
	end   int     // byte offset just after the end tag in the source
}

func (n *node) attr(name string) string { return n.attrs[name] }

// elements returns the child elements, skipping text pseudo-nodes.
func (n *node) elements() []*node {
	out := make([]*node, 0, len(n.kids))
	for _, k := range n.kids {
		if k.name != "" {
			out = append(out, k)
		}
	}
	return out
}

// child returns the first child element with the given name, or nil.
func (n *node) child(name string) *node {
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
	}
	return nil
}

// entities lists the named character references the official DTDs use that
// encoding/xml does not know; the bill DTD itself is not loaded.
var entities = map[string]string{ //nolint:gochecknoglobals // read-only table
	"nbsp": " ", "mdash": "—", "ndash": "–", "sect": "§",
	"para": "¶", "ldquo": "“", "rdquo": "”", "lsquo": "‘",
	"rsquo": "’", "hellip": "…", "times": "×", "deg": "°",
}

// buildTree reads src into a tree. Character data between elements is kept
// exactly; the offsets let non-prose elements keep their raw markup.
func buildTree(src []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = true
	dec.Entity = entities
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }

	root := &node{}
	stack := []*node{root}
	var prev int
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml: %w", err)
		}
		cur := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) > maxDepth {
				return nil, fmt.Errorf("%w (more than %d levels)", errTooDeep, maxDepth)
			}
			n := &node{name: t.Name.Local, attrs: make(map[string]string, len(t.Attr)), start: prev}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			cur.kids = append(cur.kids, n)
			stack = append(stack, n)
		case xml.EndElement:
			cur.end = int(dec.InputOffset())
			stack = stack[:len(stack)-1]
		case xml.CharData:
			cur.kids = append(cur.kids, &node{text: string(t)})
		}
		prev = int(dec.InputOffset())
	}
	if len(stack) != 1 {
		return nil, errUnfinished
	}
	return root, nil
}
