package backupper

import (
	crdbModels "github.com/metal-toolbox/governor-api/internal/models/crdb"
	psqlModels "github.com/metal-toolbox/governor-api/internal/models/psql"
	"go.uber.org/zap"
)

type sortable struct {
	dbmodel interface{}
	id      string
	parent  string
}

func (b *Backupper) crdbGroupsToSortable(in crdbModels.GroupSlice) []*sortable {
	out := []*sortable{}

	for _, g := range in {
		sg := &sortable{
			dbmodel: g,
			id:      g.ID,
		}

		if g.ApproverGroup.Valid {
			sg.parent = g.ApproverGroup.String
		}

		out = append(out, sg)
	}

	return out
}

func (b *Backupper) psqlGroupsToSortable(in psqlModels.GroupSlice) []*sortable {
	out := []*sortable{}

	for _, g := range in {
		sg := &sortable{
			dbmodel: g,
			id:      g.ID,
		}

		if g.ApproverGroup.Valid {
			sg.parent = g.ApproverGroup.String
		}

		out = append(out, sg)
	}

	return out
}

// sortPSQLGroups sorts PSQL groups topologically and returns the correctly typed slice
func (b *Backupper) sortPSQLGroups(in []*sortable) psqlModels.GroupSlice {
	sorted := b.sort(in)
	if sorted == nil {
		return nil
	}

	result := make(psqlModels.GroupSlice, 0, len(sorted))

	for _, g := range sorted {
		if group, ok := g.dbmodel.(*psqlModels.Group); ok {
			result = append(result, group)
		}
	}

	return result
}

// sortCRDBGroups sorts CRDB groups topologically and returns the correctly typed slice
func (b *Backupper) sortCRDBGroups(in []*sortable) crdbModels.GroupSlice {
	sorted := b.sort(in)
	if sorted == nil {
		return nil
	}

	result := make(crdbModels.GroupSlice, 0, len(sorted))

	for _, g := range sorted {
		if group, ok := g.dbmodel.(*crdbModels.Group); ok {
			result = append(result, group)
		}
	}

	return result
}

// sort sorts the groups topologically based on their dependencies.
func (b *Backupper) sort(in []*sortable) []*sortable {
	existsMap := make(map[string]*sortable)
	for _, g := range in {
		existsMap[g.id] = g
	}

	var (
		visited      = make(map[string]bool)
		recursionMap = make(map[string]bool)
		result       = []*sortable{}
		dfs          func(group *sortable) bool
	)

	// topological sort with DFS
	dfs = func(node *sortable) bool {
		if recursionMap[node.id] {
			b.logger.Warn("Detected cycle in group dependencies", zap.String("group_id", node.id))
			return false
		}

		if visited[node.id] {
			return true
		}

		visited[node.id] = true
		recursionMap[node.id] = true

		// if node has a non-empty parent and exists in map
		if node.parent != "" {
			if m, exists := existsMap[node.parent]; exists {
				if !dfs(m) {
					// circular dependency detected
					recursionMap[node.id] = false
					return false
				}
			}
		}

		recursionMap[node.id] = false

		result = append(result, node)

		return true
	}

	for _, node := range in {
		if !visited[node.id] {
			if !dfs(node) {
				return nil
			}
		}
	}

	return result
}
