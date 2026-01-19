package main

import (
	"DreamBiteApp/oscquery"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestName(t *testing.T) {
	file, err := os.Open("oscjson.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	var tree node
	err = json.NewDecoder(file).Decode(&tree)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(tree.find("/avatar/change"))
	spew.Dump(tree.find("/avatar/parameters/DreamBite/Grab"))
	spew.Dump(tree.find("/avatar/parameters/DreamBite/Marker"))
}

type node struct {
	FullPath    string           `json:"FULL_PATH"`
	Contents    map[string]*node `json:"CONTENTS,omitempty" exhaustruct:"optional"`
	Access      oscquery.Access  `json:"ACCESS,omitempty" exhaustruct:"optional"`
	Type        oscquery.Type    `json:"TYPE,omitempty" exhaustruct:"optional"`
	Value       []any            `json:"VALUE,omitempty" exhaustruct:"optional"`
	Description string           `json:"DESCRIPTION,omitempty" exhaustruct:"optional"`
}

func (t *node) find(fullPath string) (*node, bool) {
	if fullPath == "/" || fullPath == "" {
		return t, true
	}
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	current := t
	for _, part := range parts {
		next, ok := current.Contents[part]
		if !ok {
			return nil, false
		}

		current = next
	}

	return current, true
}
