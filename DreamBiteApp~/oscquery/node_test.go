package oscquery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNode(t *testing.T) {
	t.Parallel()

	tree := NewNodeTree()

	root, ok := tree.Find("/")
	require.True(t, ok, "Expected root node to exist")
	require.Equal(t, tree, root)

	// Add node with subnode.
	n1 := &Node{
		FullPath: "/test/osc/query/add/node",
		Contents: map[string]*Node{
			"subnode1": {
				FullPath:    "/test/osc/query/add/node/subnode1",
				Access:      AccessRead,
				Type:        TypeFloat,
				Value:       []any{0.0},
				Description: "Test subnode1",
			},
		},
		Access:      AccessReadWrite,
		Type:        TypeBool,
		Value:       []any{false},
		Description: "Test endpoint",
	}
	err := tree.Add(n1)
	require.NoError(t, err)

	testNode, ok := tree.Find("/test/osc/query/add/node")
	require.True(t, ok, "Expected node to exist")
	testSubNode1, ok := tree.Find("/test/osc/query/add/node/subnode1")
	require.True(t, ok, "Expected subnode1 to exist")

	require.Equal(t, n1, testNode)
	require.Len(t, testNode.Contents, 1)
	require.Equal(t, n1.Contents["subnode1"], testSubNode1)

	// Add subnode.
	n2 := &Node{
		FullPath:    "/test/osc/query/add/node/subnode2",
		Access:      AccessWrite,
		Type:        TypeInt,
		Value:       []any{42},
		Description: "Test subnode2",
	}
	err = tree.Add(n2)
	require.NoError(t, err)

	testSubNode2, ok := tree.Find("/test/osc/query/add/node/subnode2")
	require.True(t, ok, "Expected subnode2 to exist")

	require.Equal(t, n2, testSubNode2)
	require.Len(t, testNode.Contents, 2)
	require.Equal(t, testSubNode1, testNode.Contents["subnode1"])
	require.Equal(t, testSubNode2, testNode.Contents["subnode2"])
}
