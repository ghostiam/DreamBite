package oscquery

import (
	"reflect"
	"testing"
)

func Test_nodeTree(t *testing.T) {
	t.Parallel()

	tree := newNodeTree[testHandler]()

	n, ok := tree.find("/")
	if !ok {
		t.Fatal("Expected root node to exist")
	}
	if n != tree.root {
		t.Fatal("Expected root node to be returned")
	}

	ep := &Endpoint[testHandler]{
		FullPath:     "/test/osc/query/add/endpoint",
		Access:       AccessReadWrite,
		Type:         TypeBool,
		DefaultValue: []any{false},
		Description:  "Test endpoint",
		Handler:      func() {},
	}
	err := tree.add(ep)
	if err != nil {
		t.Fatal(err)
	}

	testEp, ok := tree.find("/test/osc/query/add/endpoint")
	if !ok {
		t.Fatal("Expected endpoint node to exist")
	}

	if testEp.FullPath != ep.FullPath {
		t.Fatal("Expected endpoint full path to match")
	}
	if testEp.Access != ep.Access {
		t.Fatal("Expected endpoint access to match")
	}
	if testEp.Type != ep.Type {
		t.Fatal("Expected endpoint type to match")
	}
	if !reflect.DeepEqual(testEp.Value, ep.DefaultValue) {
		t.Fatal("Expected endpoint default value to match")
	}
	if testEp.Description != ep.Description {
		t.Fatal("Expected endpoint description to match")
	}
	if !equalFunc(testEp.handler, ep.Handler) {
		t.Fatal("Expected endpoint handler to match")
	}
}

func equalFunc(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)
	if v1.Type() != v2.Type() {
		return false
	}

	return v1.Pointer() == v2.Pointer()
}

type testHandler func()
