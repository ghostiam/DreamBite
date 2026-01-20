package main

import (
	"encoding/json"
	"os"
	"testing"

	"DreamBiteApp/oscquery"

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

	var tree oscquery.Node
	err = json.NewDecoder(file).Decode(&tree)
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(tree.Find("/avatar/change"))
	spew.Dump(tree.Find("/avatar/parameters/DreamBite/Grab"))
	spew.Dump(tree.Find("/avatar/parameters/DreamBite/Marker"))
}
