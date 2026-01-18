package oscquery

import (
	"fmt"
	"strings"
)

type Node[Handler any] interface {
	SetValue(value []any)
	Handler() Handler
}

//nolint:tagliatelle
type node[Handler any] struct {
	FullPath    string                    `json:"FULL_PATH"`
	Contents    map[string]*node[Handler] `json:"CONTENTS,omitempty" exhaustruct:"optional"`
	Access      Access                    `json:"ACCESS,omitempty" exhaustruct:"optional"`
	Type        Type                      `json:"TYPE,omitempty" exhaustruct:"optional"`
	Value       []any                     `json:"VALUE,omitempty" exhaustruct:"optional"`
	Description string                    `json:"DESCRIPTION,omitempty" exhaustruct:"optional"`
	handler     Handler                   `exhaustruct:"optional"`
}

func (n *node[Handler]) SetValue(value []any) {
	if value == nil {
		n.Value = nil
		return
	}

	n.Value = make([]any, len(value))
	copy(n.Value, value)
}

func (n *node[Handler]) Handler() Handler {
	return n.handler
}

type nodeTree[Handler any] struct {
	root *node[Handler]
}

func newNodeTree[Handler any]() *nodeTree[Handler] {
	return &nodeTree[Handler]{
		root: &node[Handler]{FullPath: "/"},
	}
}

func (t *nodeTree[Handler]) add(ep *Endpoint[Handler]) error {
	err := ep.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	parts := strings.Split(strings.Trim(ep.FullPath, "/"), "/")
	current := t.root
	for _, part := range parts {
		if current.Contents == nil {
			current.Contents = map[string]*node[Handler]{}
		}

		next, ok := current.Contents[part]
		if !ok {
			combinedPath := current.FullPath + "/" + part
			if strings.HasSuffix(current.FullPath, "/") {
				combinedPath = current.FullPath + part
			}

			next = &node[Handler]{
				FullPath: combinedPath,
			}
		}

		current.Contents[part] = next

		current = next
	}

	current.Access = ep.Access
	current.Type = ep.Type
	current.Value = ep.DefaultValue
	current.Description = ep.Description
	current.handler = ep.Handler

	return nil
}

func (t *nodeTree[Handler]) find(fullPath string) (*node[Handler], bool) {
	if fullPath == "/" || fullPath == "" {
		return t.root, true
	}
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	current := t.root
	for _, part := range parts {
		next, ok := current.Contents[part]
		if !ok {
			return nil, false
		}

		current = next
	}

	return current, true
}
