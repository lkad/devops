package project

import (
	"sort"

	"github.com/devops-toolkit/backend/internal/database"
)

// ProjectNode is a node in the response tree returned by the
// GET /projects/:id handler. It pairs a Project with its direct
// children; grandchildren live under their parent's Children
// slice. The struct is JSON-serialisable: Project is the
// payload, Children is the recursive list.
type ProjectNode struct {
	Project  Project        `json:"project"`
	Children []*ProjectNode `json:"children"`
}

// BuildTree rearranges a flat list of Projects into a tree rooted
// at the supplied rootID. Rows that are not descendants of root
// (and are not root itself) are silently ignored — this lets the
// handler pass the entire result of a List query without having
// to filter server-side.
//
// The function returns nil when:
//   - the input list is empty, or
//   - the rootID is not present in the list.
//
// Children at every level are sorted by Name (case-sensitive) so
// the UI does not flicker between renders.
func BuildTree(projects []Project, rootID string) *ProjectNode {
	if len(projects) == 0 {
		return nil
	}
	// Build a parentID -> children map once; the cost is O(n) and
	// the slice iteration order is irrelevant because we sort
	// each list before returning.
	byParent := make(map[string][]*Project, len(projects))
	var root *Project
	all := make(map[string]*Project, len(projects))
	for i := range projects {
		p := &projects[i]
		all[p.ID] = p
		if p.ID == rootID {
			root = p
		}
	}
	if root == nil {
		return nil
	}
	for i := range projects {
		p := &projects[i]
		if p.ParentID == nil {
			continue
		}
		byParent[*p.ParentID] = append(byParent[*p.ParentID], p)
	}
	return buildNode(root, byParent)
}

// buildNode is the recursive builder. Each call consumes the
// children list for one parent and recursively builds their
// subtrees. The result is a leaf-rooted ProjectNode.
func buildNode(p *Project, byParent map[string][]*Project) *ProjectNode {
	node := &ProjectNode{Project: *p}
	kids := byParent[p.ID]
	if len(kids) == 0 {
		return node
	}
	sort.Slice(kids, func(i, j int) bool {
		return kids[i].Name < kids[j].Name
	})
	for _, k := range kids {
		node.Children = append(node.Children, buildNode(k, byParent))
	}
	return node
}

// zeroBase is a test-only helper that builds a BaseModel with a
// caller-supplied ID and no timestamps. Lives in this file
// (rather than the testhelpers file) so it sits next to its only
// caller without leaking the construct into production code.
func zeroBase(id string) database.BaseModel {
	return database.BaseModel{ID: id}
}
