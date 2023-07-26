package dbtools

import (
	"context"
	"database/sql"
	"errors"

	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// allMembershipsQuery works by fetching an "initial state" consisting of all memberships in the `group_memberships` table (direct memberships).
// This is unioned and recursively joined with the `group_hierarchies` table, such that those group memberships are also applied to any "parent"
// groups as defined hierarchically, called "indirect memberships". A user cannot be an admin of a group via indirect membership. Then, a
// GROUP BY is applied on `group_id` and `user_id`, so that multiple paths of membership are collapsed into one entry, and `is_admin` and
// `direct` are aggregated together using a boolean OR so that the values from the direct membership are preferred (if they exist).

// membershipsByUserQuery works almost exactly the same way as allMembershipsQuery, but applies a WHERE clause filtering on `user_id` to the
// very first "initial state" query. This works because we know that all paths to direct or indirect membership begin with an entry in the
// `group_memberships` table.

// membershipsByGroupQuery is the odd one out, and does not work the same way as the others. This is the case because, when beginning with a
// group, you have to look at both the `group_memberships` table and the `group_hierarchies` table, but are not guaranteed to always find records
// in either (though valid group memberships may still exist). Therefore it works by first using a recursive query to attempt to build a tree
// out of records in the `group_hierarchies` table, with this group as the root node (this is the `hierarchical_groups` CTE). Then, in case there
// are no records as a result of that (this group contains no subgroups), that result is UNIONed with a single row containing at least this group
// as a root (this is the `ensure_root` CTE). The result of that tree generation is joined with `group_memberships`, and `is_admin` and
// `expires_at` are only considered if the membership is a direct one and not an indirect one.

// All of these queries _could_ work just by running allMembershipsQuery with a `WHERE user_id = ?` or `WHERE group_id = ?` on the end (and
// that is a good way to debug, to ensure the results are the same), however, CRDB does not perform "predicate pushdown" and optimize the
// recursive queries based on those conditions, and thus will always perform a full table scan to execute the query when run that way. These
// queries were written and parameterized this way for performance reasons to avoid full table scans every time membership is enumerated.

const (
	allMembershipsQuery = `WITH RECURSIVE membership_query AS (
		SELECT
			group_id,
			user_id,
			expires_at,
			is_admin,
			TRUE AS direct
		FROM
			group_memberships
		UNION ALL
		SELECT
			b.parent_group_id,
			a.user_id,
			b.expires_at,
			FALSE AS is_admin,
			FALSE AS direct
		FROM
			membership_query AS a
			INNER JOIN group_hierarchies AS b ON a.group_id = b.member_group_id
	)
	SELECT
		group_id,
		user_id,
		CASE WHEN BOOL_OR(direct) THEN
			MAX(expires_at)
		ELSE
			NULL
		END AS expires_at,
		BOOL_OR(is_admin) as is_admin,
		BOOL_OR(direct) as direct
	FROM
		membership_query
	GROUP BY
		group_id,
		user_id;`
	membershipsByUserQuery = `WITH RECURSIVE membership_query AS (
		SELECT
			group_id,
			user_id,
			is_admin,
			expires_at,
			TRUE AS direct
		FROM
			group_memberships
		WHERE user_id = $1
		UNION ALL
		SELECT
			b.parent_group_id,
			a.user_id,
			FALSE AS is_admin,
			NULL as expires_at,
			FALSE AS direct
		FROM
			membership_query AS a
			INNER JOIN group_hierarchies AS b ON a.group_id = b.member_group_id
	)
	SELECT
		group_id,
		user_id,
		CASE WHEN BOOL_OR(direct) THEN
			MAX(expires_at)
		ELSE
			NULL
		END AS expires_at,
		BOOL_OR(is_admin) as is_admin,
		BOOL_OR(direct) as direct
	FROM
		membership_query
	GROUP BY
		group_id,
		user_id;`
	membershipsByGroupQuery = `WITH RECURSIVE hierarchical_groups AS (
		SELECT
			parent_group_id AS group_id,
			member_group_id AS comparator,
			TRUE AS direct
		FROM
			group_hierarchies
		WHERE
			parent_group_id = $1
		UNION
		SELECT
			b.member_group_id AS group_id,
			a.group_id AS comparator,
			FALSE AS direct
		FROM
			hierarchical_groups AS a
			INNER JOIN group_hierarchies AS b ON a.group_id = b.parent_group_id
	),
	ensure_root AS (
		SELECT
			$1 AS group_id,
			NULL AS comparator,
			TRUE AS direct
		UNION
		SELECT
			*
		FROM
			hierarchical_groups
	)
	SELECT DISTINCT
		$1 AS group_id,
		user_id,
		BOOL_OR(CASE WHEN direct THEN
			group_memberships.is_admin
		ELSE
			FALSE
		END) AS is_admin,
		CASE WHEN BOOL_OR(direct) THEN
			MAX(group_memberships.expires_at)
		ELSE
			NULL
		END AS expires_at,
		BOOL_OR(direct) as direct
	FROM
		ensure_root
		INNER JOIN group_memberships ON group_memberships.group_id = ensure_root.group_id
	GROUP BY
		group_memberships.user_id;`
)

// EnumeratedMembership represents a single user-to-group membership, which may be direct or indirect
type EnumeratedMembership struct {
	GroupID   string
	Group     *models.Group
	UserID    string
	User      *models.User
	IsAdmin   bool
	ExpiresAt null.Time
	Direct    bool
}

// GetMembershipsForUser returns a fully enumerated list of memberships for a user, optionally with sqlboiler's generated models populated
func GetMembershipsForUser(ctx context.Context, db *sql.DB, userID string, shouldPopulateAllModels bool) ([]EnumeratedMembership, error) {
	enumeratedMemberships := []EnumeratedMembership{}

	err := queries.Raw(membershipsByUserQuery, userID).Bind(ctx, db, &enumeratedMemberships)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	if shouldPopulateAllModels {
		enumeratedMemberships, err = populateModels(ctx, db, enumeratedMemberships)
		if err != nil {
			return nil, err
		}
	}

	return enumeratedMemberships, nil
}

// GetMembersOfGroup returns a fully enumerated list of memberships in a group, optionally with sqlboiler's generated models populated
func GetMembersOfGroup(ctx context.Context, db *sql.DB, groupID string, shouldPopulateAllModels bool) ([]EnumeratedMembership, error) {
	enumeratedMemberships := []EnumeratedMembership{}

	err := queries.Raw(membershipsByGroupQuery, groupID).Bind(ctx, db, &enumeratedMemberships)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return []EnumeratedMembership{}, err
		}
	}

	if shouldPopulateAllModels {
		enumeratedMemberships, err = populateModels(ctx, db, enumeratedMemberships)
		if err != nil {
			return []EnumeratedMembership{}, err
		}
	}

	return enumeratedMemberships, nil
}

// GetAllGroupMemberships returns a fully enumerated list of all memberships in the database, optionally with sqlboiler's generated models populated (use with caution, potentially lots of data)
func GetAllGroupMemberships(ctx context.Context, db *sql.DB, shouldPopulateAllModels bool) ([]EnumeratedMembership, error) {
	enumeratedMemberships := []EnumeratedMembership{}

	err := queries.Raw(allMembershipsQuery).Bind(ctx, db, &enumeratedMemberships)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return []EnumeratedMembership{}, err
		}
	}

	if shouldPopulateAllModels {
		enumeratedMemberships, err = populateModels(ctx, db, enumeratedMemberships)
		if err != nil {
			return []EnumeratedMembership{}, err
		}
	}

	return enumeratedMemberships, nil
}

// CheckNewHierarchyWouldCreateCycle ensures that a new hierarchy does not create a loop or cycle in the database
func CheckNewHierarchyWouldCreateCycle(ctx context.Context, db *sql.DB, parentGroupID, memberGroupID string) (bool, error) {
	hierarchies := make(map[string][]string)

	hierarchyRows, err := models.GroupHierarchies().All(ctx, db)
	if err != nil {
		return false, err
	}

	for _, row := range hierarchyRows {
		hierarchies[row.ParentGroupID] = append(hierarchies[row.ParentGroupID], row.MemberGroupID)
	}

	hierarchies[parentGroupID] = append(hierarchies[parentGroupID], memberGroupID)

	var walkNode func(startingID string, hierarchies map[string][]string, visited []string) bool
	walkNode = func(startingID string, hierarchies map[string][]string, visited []string) bool {
		if _, exists := hierarchies[startingID]; !exists {
			return false
		}

		if contains(visited, startingID) {
			return true
		}

		for _, e := range hierarchies[startingID] {
			if walkNode(e, hierarchies, append(visited, startingID)) {
				return true
			}
		}

		return false
	}

	for i := range hierarchies {
		if walkNode(i, hierarchies, []string{}) {
			return true, nil
		}
	}

	return false, nil
}

// FindMemberDiff finds members present in the second EnumeratedMembership which are not present in the first
func FindMemberDiff(before, after []EnumeratedMembership) []EnumeratedMembership {
	beforeMap := make(map[EnumeratedMembership]bool)

	for _, e := range before {
		beforeMap[e] = true
	}

	uniqueMembersAfter := make([]EnumeratedMembership, 0)

	for _, e := range after {
		if _, exists := beforeMap[e]; !exists {
			uniqueMembersAfter = append(uniqueMembersAfter, e)
		}
	}

	return uniqueMembersAfter
}

func populateModels(ctx context.Context, db *sql.DB, memberships []EnumeratedMembership) ([]EnumeratedMembership, error) {
	groupIDSet := make(map[string]bool)
	userIDSet := make(map[string]bool)

	for _, m := range memberships {
		if _, exists := groupIDSet[m.GroupID]; !exists {
			groupIDSet[m.GroupID] = true
		}

		if _, exists := userIDSet[m.UserID]; !exists {
			userIDSet[m.UserID] = true
		}
	}

	groupIDs := stringMapToKeySlice(groupIDSet)
	userIDs := stringMapToKeySlice(userIDSet)

	queryMods := []qm.QueryMod{
		qm.WhereIn("id in ?", stringSliceToInterface(groupIDs)...),
	}

	groups, err := models.Groups(queryMods...).All(ctx, db)
	if err != nil {
		return []EnumeratedMembership{}, err
	}

	queryMods = []qm.QueryMod{
		qm.WhereIn("id in ?", stringSliceToInterface(userIDs)...),
	}

	users, err := models.Users(queryMods...).All(ctx, db)
	if err != nil {
		return []EnumeratedMembership{}, err
	}

	for i, m := range memberships {
		memberships[i].Group = findGroupByID(groups, m.GroupID)
		memberships[i].User = findUserByID(users, m.UserID)
	}

	return memberships, nil
}

func findGroupByID(list models.GroupSlice, id string) *models.Group {
	for _, i := range list {
		if i.ID == id {
			return i
		}
	}

	return nil
}

func findUserByID(list models.UserSlice, id string) *models.User {
	for _, i := range list {
		if i.ID == id {
			return i
		}
	}

	return nil
}

func stringSliceToInterface(s []string) []interface{} {
	convertedIDs := make([]interface{}, len(s))

	for i, str := range s {
		convertedIDs[i] = str
	}

	return convertedIDs
}

func stringMapToKeySlice(m map[string]bool) []string {
	keys := make([]string, len(m))

	i := 0

	for k := range m {
		keys[i] = k
		i++
	}

	return keys
}
