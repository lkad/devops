package project

// TestTree_BuildTree_HappyPath covers the spec scenario
// "Get Business Line with nested systems" and "Get System with
// nested projects": a flat slice of Projects is reorganised into
// a ProjectNode tree rooted at the supplied rootID. Children of
// each node must be sorted by name for stable UI rendering.
import "testing"

func TestTree_BuildTree_HappyPath(t *testing.T) {
	pt := ProjectType{BaseModel: zeroBase("t1"), Name: "platform"}

	bl := Project{BaseModel: zeroBase("p1"), Name: "BL", Code: "bl", TypeID: pt.ID}
	s1 := Project{BaseModel: zeroBase("p2"), Name: "Sys1", Code: "s1", TypeID: pt.ID, ParentID: &bl.ID}
	s2 := Project{BaseModel: zeroBase("p3"), Name: "Sys2", Code: "s2", TypeID: pt.ID, ParentID: &bl.ID}
	pr1 := Project{BaseModel: zeroBase("p4"), Name: "Proj1", Code: "pr1", TypeID: pt.ID, ParentID: &s1.ID}
	pr2 := Project{BaseModel: zeroBase("p5"), Name: "Proj2", Code: "pr2", TypeID: pt.ID, ParentID: &s1.ID}
	pr3 := Project{BaseModel: zeroBase("p6"), Name: "Proj3", Code: "pr3", TypeID: pt.ID, ParentID: &s2.ID}

	tree := BuildTree([]Project{bl, s1, s2, pr1, pr2, pr3}, bl.ID)
	if tree == nil {
		t.Fatal("expected non-nil root")
	}
	if tree.Project.ID != bl.ID {
		t.Errorf("root id=%s want %s", tree.Project.ID, bl.ID)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("root children=%d want 2", len(tree.Children))
	}
	// children must be sorted by name; Sys1 first, Sys2 second
	if tree.Children[0].Project.Name != "Sys1" || tree.Children[1].Project.Name != "Sys2" {
		t.Errorf("sort: %s %s", tree.Children[0].Project.Name, tree.Children[1].Project.Name)
	}
	// Sys1 should have Proj1 and Proj2
	sys1 := tree.Children[0]
	if len(sys1.Children) != 2 {
		t.Errorf("Sys1 children=%d want 2", len(sys1.Children))
	}
}

// TestTree_BuildTree_OnlyIncludesDescendants ensures nodes from
// unrelated branches are not pulled in. The BuildTree function
// must stop walking at the root, not at the entire dataset.
func TestTree_BuildTree_OnlyIncludesDescendants(t *testing.T) {
	pt := ProjectType{BaseModel: zeroBase("t1"), Name: "platform"}
	bl1 := Project{BaseModel: zeroBase("p1"), Name: "BL1", Code: "bl1", TypeID: pt.ID}
	bl2 := Project{BaseModel: zeroBase("p2"), Name: "BL2", Code: "bl2", TypeID: pt.ID}
	s := Project{BaseModel: zeroBase("p3"), Name: "Sys", Code: "s", TypeID: pt.ID, ParentID: &bl1.ID}
	tree := BuildTree([]Project{bl1, bl2, s}, bl1.ID)
	if tree == nil {
		t.Fatal("expected non-nil root")
	}
	if len(tree.Children) != 1 || tree.Children[0].Project.ID != s.ID {
		t.Errorf("expected only s under bl1, got %+v", tree.Children)
	}
}

// TestTree_BuildTree_EmptyList verifies the nil root contract.
func TestTree_BuildTree_EmptyList(t *testing.T) {
	if got := BuildTree(nil, "x"); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

// TestTree_BuildTree_UnknownRoot is the negative path: the root
// id is not in the slice. We return nil rather than guessing.
func TestTree_BuildTree_UnknownRoot(t *testing.T) {
	pt := ProjectType{BaseModel: zeroBase("t1"), Name: "platform"}
	bl := Project{BaseModel: zeroBase("p1"), Name: "BL", Code: "bl", TypeID: pt.ID}
	if got := BuildTree([]Project{bl}, "nope"); got != nil {
		t.Errorf("expected nil for unknown root, got %+v", got)
	}
}

// TestTree_BuildTree_LeafNoChildren covers a single-leaf tree: the
// root has no children but the function still returns a node.
func TestTree_BuildTree_LeafNoChildren(t *testing.T) {
	pt := ProjectType{BaseModel: zeroBase("t1"), Name: "platform"}
	bl := Project{BaseModel: zeroBase("p1"), Name: "BL", Code: "bl", TypeID: pt.ID}
	tree := BuildTree([]Project{bl}, bl.ID)
	if tree == nil {
		t.Fatal("expected non-nil root")
	}
	if len(tree.Children) != 0 {
		t.Errorf("expected no children, got %d", len(tree.Children))
	}
}

// TestTree_BuildTree_ChildrenSorted is a focused regression: child
// order must always be by name regardless of input order, so the
// UI does not flicker between renders.
func TestTree_BuildTree_ChildrenSorted(t *testing.T) {
	pt := ProjectType{BaseModel: zeroBase("t1"), Name: "platform"}
	bl := Project{BaseModel: zeroBase("p1"), Name: "BL", Code: "bl", TypeID: pt.ID}
	c1 := Project{BaseModel: zeroBase("p2"), Name: "zeta", Code: "z", TypeID: pt.ID, ParentID: &bl.ID}
	c2 := Project{BaseModel: zeroBase("p3"), Name: "alpha", Code: "a", TypeID: pt.ID, ParentID: &bl.ID}
	c3 := Project{BaseModel: zeroBase("p4"), Name: "Mid", Code: "m", TypeID: pt.ID, ParentID: &bl.ID}
	tree := BuildTree([]Project{bl, c1, c2, c3}, bl.ID)
	if tree == nil || len(tree.Children) != 3 {
		t.Fatalf("expected 3 children, got %+v", tree)
	}
	want := []string{"Mid", "alpha", "zeta"}
	for i, w := range want {
		if tree.Children[i].Project.Name != w {
			t.Errorf("child[%d]=%s want %s", i, tree.Children[i].Project.Name, w)
		}
	}
}
