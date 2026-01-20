package oscquery

import (
	"errors"
	"fmt"
	"strings"
)

//nolint:tagliatelle
type Node struct {
	FullPath    string           `json:"FULL_PATH"`
	Contents    map[string]*Node `json:"CONTENTS,omitempty" exhaustruct:"optional"`
	Access      Access           `json:"ACCESS,omitempty" exhaustruct:"optional"`
	Type        Type             `json:"TYPE,omitempty" exhaustruct:"optional"`
	Value       []any            `json:"VALUE,omitempty" exhaustruct:"optional"`
	Description string           `json:"DESCRIPTION,omitempty" exhaustruct:"optional"`
}

func NewNodeTree() *Node {
	return &Node{FullPath: "/"}
}

func (n *Node) Validate() error {
	if n.FullPath == "" {
		return errors.New("path cannot be empty")
	}
	if n.Access == AccessNone {
		return errors.New("access cannot be none")
	}
	if n.Type == "" {
		return errors.New("type cannot be empty")
	}

	for key, node := range n.Contents {
		err := node.Validate()
		if err != nil {
			return errors.New("subnode " + key + ": " + err.Error())
		}
	}

	return nil
}

func (n *Node) Add(v *Node) error {
	parts := strings.Split(strings.Trim(v.FullPath, "/"), "/")
	current := n
	for _, part := range parts {
		if current.Contents == nil {
			current.Contents = map[string]*Node{}
		}

		next, ok := current.Contents[part]
		if !ok {
			combinedPath := current.FullPath + "/" + part
			if strings.HasSuffix(current.FullPath, "/") {
				combinedPath = current.FullPath + part
			}

			next = &Node{
				FullPath: combinedPath,
			}
		}

		current.Contents[part] = next

		current = next
	}

	current.Access = v.Access
	current.Type = v.Type
	current.Value = v.Value
	current.Description = v.Description

	for key, node := range v.Contents {
		err := n.Add(node)
		if err != nil {
			return fmt.Errorf("add subnode %s: %w", key, err)
		}
	}

	return nil
}

func (n *Node) Find(fullPath string) (*Node, bool) {
	if fullPath == "/" || fullPath == "" {
		return n, true
	}
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	current := n
	for _, part := range parts {
		next, ok := current.Contents[part]
		if !ok {
			return nil, false
		}

		current = next
	}

	return current, true
}
