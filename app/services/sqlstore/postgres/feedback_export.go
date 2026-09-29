package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/bits"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

type dbFeedbackExportPreset struct {
	ID        string         `db:"id"`
	Name      sql.NullString `db:"name"`
	Recipe    sql.NullString `db:"recipe"`
	UpdatedAt time.Time      `db:"updated_at"`
	UpdatedBy string         `db:"updated_by"`
}

func (preset *dbFeedbackExportPreset) toModel() (*entity.FeedbackExportPreset, error) {
	if !preset.Name.Valid {
		return nil, app.ErrNotFound
	}

	result := &entity.FeedbackExportPreset{
		ID:        preset.ID,
		Name:      preset.Name.String,
		UpdatedAt: preset.UpdatedAt,
		UpdatedBy: preset.UpdatedBy,
	}

	if err := json.Unmarshal([]byte(preset.Recipe.String), &result.Recipe); err != nil {
		return nil, errors.Wrap(err, "failed to parse feedback export preset %s", preset.ID)
	}
	result.Recipe.CanonicalizeFilters()

	return result, nil
}

type dbFeedbackExportTag struct {
	PostID int64 `db:"post_id"`
	TagID  int   `db:"tag_id"`
}

type dbFeedbackExportRow struct {
	ID       int64  `db:"id"`
	Number   int    `db:"number"`
	Title    string `db:"title"`
	Slug     string `db:"slug"`
	Votes    int    `db:"votes"`
	Comments int    `db:"comments_count"`
}

func listFeedbackExportPresets(ctx context.Context, q *query.ListFeedbackExportPresets) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var presets []*dbFeedbackExportPreset
		err := trx.Select(&presets, `
			SELECT p.id, p.name, p.recipe, p.updated_at, COALESCE(u.name, '') AS updated_by
			FROM feedback_export_presets p
			LEFT JOIN users u ON u.id = p.updated_by_id AND u.tenant_id = p.tenant_id
			WHERE p.tenant_id = $1 AND p.name IS NOT NULL
			ORDER BY lower(p.name)`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to list feedback export presets")
		}

		q.Result = make([]*entity.FeedbackExportPreset, 0, len(presets))
		for _, preset := range presets {
			result, err := preset.toModel()
			if err != nil {
				return err
			}
			q.Result = append(q.Result, result)
		}
		return nil
	})
}

func readFeedbackExportPreset(trx *dbx.Trx, tenantID int, id string) (*dbFeedbackExportPreset, error) {
	var preset dbFeedbackExportPreset
	err := trx.Get(&preset, `
				SELECT p.id, p.name, p.recipe, p.updated_at, COALESCE(u.name, '') AS updated_by
		FROM feedback_export_presets p
		LEFT JOIN users u ON u.id = p.updated_by_id AND u.tenant_id = p.tenant_id
		WHERE p.tenant_id = $1 AND p.id = $2
	`, tenantID, id)
	return &preset, err
}

func feedbackExportNameConflict() *validate.Result {
	result := validate.Success()
	result.AddFieldFailure("name", "A preset with this name already exists.")
	return result
}

func createFeedbackExportPreset(ctx context.Context, c *cmd.CreateFeedbackExportPreset) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ExportFeedback); err != nil {
			return err
		}

		recipe, err := json.Marshal(c.Recipe)
		if err != nil {
			return errors.Wrap(err, "failed to marshal feedback export recipe")
		}

		_, err = trx.Execute(`
			INSERT INTO feedback_export_presets (tenant_id, id, name, recipe, created_at, updated_at, updated_by_id)
			VALUES ($1, $2, $3, $4, NOW(), NOW(), $5)
			ON CONFLICT DO NOTHING
		`, tenant.ID, c.ID, c.Name, recipe, user.ID)
		if err != nil {
			return errors.Wrap(err, "failed to create feedback export preset")
		}

		preset, err := readFeedbackExportPreset(trx, tenant.ID, c.ID)
		if err == app.ErrNotFound {
			return feedbackExportNameConflict()
		}

		if err != nil {
			return err
		}

		c.Result, err = preset.toModel()
		return err
	})
}

func updateFeedbackExportPreset(ctx context.Context, c *cmd.UpdateFeedbackExportPreset) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ExportFeedback); err != nil {
			return err
		}

		fingerprint, err := json.Marshal(struct {
			ID     string
			Name   string
			Recipe entity.FeedbackExportRecipe
			Saved  entity.FeedbackExportPresetContent
		}{c.ID, c.Name, c.Recipe, c.Saved})
		if err != nil {
			return err
		}

		receipt := commandReceipt{
			TenantID: tenant.ID, UserID: user.ID, Kind: "export-preset-update",
			SubmissionID: c.SubmissionID, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(fingerprint)),
		}
		replayed, err := receipt.read(trx, nil)
		if err != nil {
			return err
		}

		var locked dbFeedbackExportPreset
		err = trx.Get(&locked, `
			SELECT p.id, p.name, p.recipe, p.updated_at, COALESCE(u.name, '') AS updated_by
			FROM feedback_export_presets p
			LEFT JOIN users u ON u.id = p.updated_by_id AND u.tenant_id = p.tenant_id
			WHERE p.tenant_id = $1 AND p.id = $2
			FOR UPDATE OF p
		`, tenant.ID, c.ID)
		if err != nil {
			return err
		}

		current, err := locked.toModel()
		if err != nil {
			return err
		}

		c.Result = &entity.FeedbackExportPresetUpdate{Preset: current, Conflicts: []string{}}
		if replayed {
			return nil
		}

		name := current.Name
		if c.Name != c.Saved.Name {
			if current.Name != c.Saved.Name && current.Name != c.Name {
				c.Result.Conflicts = append(c.Result.Conflicts, "name")
			}
			name = c.Name
		}

		mergedRecipe := current.Recipe
		if !c.Recipe.Equal(c.Saved.Recipe) {
			if !current.Recipe.Equal(c.Saved.Recipe) && !current.Recipe.Equal(c.Recipe) {
				c.Result.Conflicts = append(c.Result.Conflicts, "recipe")
			}
			mergedRecipe = c.Recipe
		}

		if len(c.Result.Conflicts) > 0 {
			return nil
		}
		if name == current.Name && mergedRecipe.Equal(current.Recipe) {
			return receipt.save(trx, nil)
		}

		recipe, err := json.Marshal(mergedRecipe)
		if err != nil {
			return errors.Wrap(err, "failed to marshal feedback export recipe")
		}

		_, err = trx.Execute(`
			UPDATE feedback_export_presets
				SET name = $3, recipe = $4, updated_at = NOW(), updated_by_id = $5
			WHERE tenant_id = $1 AND id = $2 AND name IS NOT NULL
			`, tenant.ID, c.ID, name, recipe, user.ID)
		if err != nil {
			if conflict, ok := errors.Cause(err).(*pq.Error); ok && conflict.Constraint == "feedback_export_presets_name" {
				return feedbackExportNameConflict()
			}

			return errors.Wrap(err, "failed to update feedback export preset")
		}

		preset, err := readFeedbackExportPreset(trx, tenant.ID, c.ID)
		if err != nil {
			return err
		}

		c.Result.Preset, err = preset.toModel()
		if err != nil {
			return err
		}
		return receipt.save(trx, nil)
	})
}

func deleteFeedbackExportPreset(ctx context.Context, c *cmd.DeleteFeedbackExportPreset) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ExportFeedback); err != nil {
			return err
		}

		result, err := trx.Execute(`
			UPDATE feedback_export_presets
			SET name = NULL, recipe = NULL, updated_by_id = NULL, updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2
		`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete feedback export preset")
		}

		if result == 0 {
			return app.ErrNotFound
		}

		return nil
	})
}

// These expressions must match the export indexes.
var feedbackExportOrders = map[entity.FeedbackExportMode]string{
	entity.FeedbackExportTopVoted:      "p.upvotes - p.downvotes DESC",
	entity.FeedbackExportDiscussed:     "p.comments_count DESC",
	entity.FeedbackExportControversial: "(p.upvotes + p.downvotes) * LEAST(p.upvotes, p.downvotes)::float8 / GREATEST(p.upvotes, p.downvotes, 1) DESC",
}

func selectFeedbackExportRows(ctx context.Context, q *query.SelectFeedbackExportRows) error {
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	viewPrivateTags := entity.Can(user, tenant, entity.ViewPrivateTags)

	if trx, _ := ctx.Value(app.TransactionCtxKey).(*dbx.Trx); trx != nil {
		return readFeedbackExportRows(trx, tenant.ID, viewPrivateTags, q)
	}

	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}

	defer trx.Rollback()

	if err := readFeedbackExportRows(trx, tenant.ID, viewPrivateTags, q); err != nil {
		return err
	}

	return trx.Commit()
}

type feedbackExportPool struct {
	Section      entity.FeedbackExportSection
	Pick         entity.FeedbackExportPick
	FirstRequest int
	FirstRow     int
	Count        int
	IDs          []int64
	Next         int
}

type feedbackExportRequest struct {
	Section int
	Count   int
	Pool    *feedbackExportPool
}

func sameFeedbackExportSelection(pool *feedbackExportPool, section entity.FeedbackExportSection, pick entity.FeedbackExportPick) bool {
	return pool.Pick.Mode == pick.Mode &&
		pool.Pick.MinComments == pick.MinComments &&
		pool.Section.SameFilters(section)
}

func feedbackExportRequests(recipe entity.FeedbackExportRecipe) []feedbackExportRequest {
	var requests []feedbackExportRequest
	var pools []*feedbackExportPool
	rows := 0

	for index, section := range recipe.Sections {
		for _, pick := range section.Picks {
			var pool *feedbackExportPool
			for _, candidate := range pools {
				if sameFeedbackExportSelection(candidate, section, pick) {
					pool = candidate
					break
				}
			}

			if pool == nil {
				pool = &feedbackExportPool{
					Section:      section,
					Pick:         pick,
					FirstRequest: len(requests),
					FirstRow:     rows,
				}
				pools = append(pools, pool)
			}

			rows += pick.Count
			// Intervening picks can consume candidates before this pool's last use.
			pool.Count = rows - pool.FirstRow
			requests = append(requests, feedbackExportRequest{Section: index, Count: pick.Count, Pool: pool})
		}
	}

	return requests
}

const feedbackExportColumns = "p.id, p.number, p.title, p.slug, p.upvotes - p.downvotes AS votes, p.comments_count"

func readFeedbackExportRows(trx *dbx.Trx, tenantID int, viewPrivateTags bool, q *query.SelectFeedbackExportRows) error {
	sections := make([][]*entity.FeedbackExportRow, len(q.Recipe.Sections))
	rowCount := 0
	for i, section := range q.Recipe.Sections {
		sectionCount := 0
		for _, pick := range section.Picks {
			sectionCount += pick.Count
		}

		sections[i] = make([]*entity.FeedbackExportRow, 0, sectionCount)
		rowCount += sectionCount
	}

	requests := feedbackExportRequests(q.Recipe)
	var randomPools []*feedbackExportPool
	poolCount := 0
	for index, request := range requests {
		if request.Pool.FirstRequest != index {
			continue
		}

		poolCount++
		if request.Pool.Pick.Mode == entity.FeedbackExportRandom {
			randomPools = append(randomPools, request.Pool)
		}
	}

	asOf := time.Now()
	singleSelection := poolCount == 1
	shareRandomOrder := len(randomPools) > 2
	var posts []*dbFeedbackExportRow
	if singleSelection {
		pool := requests[0].Pool
		pick := pool.Pick
		pick.Count = pool.Count
		statement, args := feedbackExportPickSQL(tenantID, viewPrivateTags, pool.Section, pick, q.Seed, nil, asOf)
		if err := trx.Select(&posts, "SELECT "+feedbackExportColumns+" "+statement, args...); err != nil {
			return errors.Wrap(err, "failed to read feedback export posts")
		}

		pool.IDs = make([]int64, len(posts))
		for i, post := range posts {
			pool.IDs[i] = post.ID
		}
	} else if shareRandomOrder {
		if err := selectRandomFeedbackExportPools(trx, tenantID, viewPrivateTags, randomPools, q.Seed, asOf); err != nil {
			return err
		}
	}

	rows := make([]entity.FeedbackExportRow, rowCount)
	taken := make([]int64, 0, rowCount)
	rowsByID := make(map[int64]*entity.FeedbackExportRow, rowCount)
	for index, request := range requests {
		pool := request.Pool
		if !singleSelection && index == pool.FirstRequest && !(shareRandomOrder && pool.Pick.Mode == entity.FeedbackExportRandom) {
			pick := pool.Pick
			pick.Count = pool.Count
			ids, err := selectFeedbackExportPick(trx, tenantID, viewPrivateTags, pool.Section, pick, q.Seed, taken, asOf)
			if err != nil {
				return err
			}
			pool.IDs = ids
		}

		selected := 0
		for pool.Next < len(pool.IDs) && selected < request.Count {
			id := pool.IDs[pool.Next]
			pool.Next++
			if rowsByID[id] != nil {
				continue
			}

			result := &rows[len(taken)]
			result.Pick = pool.Pick.Mode
			sections[request.Section] = append(sections[request.Section], result)
			rowsByID[id] = result
			taken = append(taken, id)
			selected++
		}
	}

	if len(taken) > 0 {
		if !singleSelection {
			err := trx.Select(&posts, "SELECT "+feedbackExportColumns+`
				FROM posts p WHERE p.tenant_id = $1 AND p.id = ANY($2)
			`, tenantID, pq.Array(taken))
			if err != nil {
				return errors.Wrap(err, "failed to read feedback export posts")
			}
		}

		for _, post := range posts {
			result := rowsByID[post.ID]
			result.Number = post.Number
			result.Title = post.Title
			result.Slug = post.Slug
			result.Votes = post.Votes
			result.Comments = post.Comments
			result.TagIDs = []int{}
		}

		var tags []*dbFeedbackExportTag
		err := trx.Select(&tags, `
			SELECT pt.post_id, pt.tag_id FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.tenant_id = $1 AND pt.post_id = ANY($2) AND (t.is_public OR $3)
			ORDER BY pt.tag_id
		`, tenantID, pq.Array(taken), viewPrivateTags)
		if err != nil {
			return errors.Wrap(err, "failed to read feedback export tags")
		}

		for _, tag := range tags {
			row := rowsByID[tag.PostID]
			row.TagIDs = append(row.TagIDs, tag.TagID)
		}
	}

	q.Result = sections
	return nil
}

func feedbackExportFilterSQL(section entity.FeedbackExportSection, pick entity.FeedbackExportPick, viewPrivateTags bool, asOf time.Time, bind func(any) string) string {
	statuses := make([]int, len(section.Statuses))
	for i, status := range section.Statuses {
		statuses[i] = int(status)
	}

	conditions := []string{
		"p.status = ANY(" + bind(pq.Array(statuses)) + ")",
		"p.comments_count >= " + bind(pick.MinComments),
	}

	if section.MinVotes != nil {
		conditions = append(conditions, "p.upvotes - p.downvotes >= "+bind(*section.MinVotes))
	}

	if section.MaxAgeDays > 0 {
		conditions = append(conditions, "p.created_at >= "+bind(asOf.AddDate(0, 0, -section.MaxAgeDays)))
	}

	if len(section.IncludeTags) > 0 {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.tenant_id = $1 AND pt.post_id = p.id AND (t.is_public OR `+bind(viewPrivateTags)+`)
				AND pt.tag_id = ANY(`+bind(pq.Array(section.IncludeTags))+`)
		)`)
	}

	if len(section.ExcludeTags) > 0 {
		conditions = append(conditions, `NOT EXISTS (
			SELECT 1 FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.tenant_id = $1 AND pt.post_id = p.id AND (t.is_public OR `+bind(viewPrivateTags)+`)
				AND pt.tag_id = ANY(`+bind(pq.Array(section.ExcludeTags))+`)
		)`)
	}

	return strings.Join(conditions, " AND ")
}

func feedbackExportPickSQL(tenantID int, viewPrivateTags bool, section entity.FeedbackExportSection, pick entity.FeedbackExportPick, seed string, taken []int64, asOf time.Time) (string, []any) {
	args := []any{tenantID}
	bind := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	// The literal status list matches the export indexes' predicate.
	statement := `FROM posts p
		WHERE p.tenant_id = $1 AND p.status IN (0, 1, 2, 3, 4, 5) AND NOT p.moderation_pending
			AND ` + feedbackExportFilterSQL(section, pick, viewPrivateTags, asOf, bind)

	if len(taken) > 0 {
		statement += ` AND NOT EXISTS (
			SELECT 1 FROM unnest(` + bind(pq.Array(taken)) + `::bigint[]) excluded(id)
			WHERE excluded.id = p.id
		)`
	}

	order, ok := feedbackExportOrders[pick.Mode]
	if pick.Mode == entity.FeedbackExportRandom {
		order, ok = "decode(md5("+bind(seed)+" || p.id::text), 'hex')", true
	}
	if !ok {
		panic(fmt.Sprintf("feedback export mode %q was not validated", pick.Mode))
	}
	statement += " ORDER BY " + order + ", p.id DESC LIMIT " + bind(pick.Count)

	return statement, args
}

func selectFeedbackExportPick(trx *dbx.Trx, tenantID int, viewPrivateTags bool, section entity.FeedbackExportSection, pick entity.FeedbackExportPick, seed string, taken []int64, asOf time.Time) ([]int64, error) {
	statement, args := feedbackExportPickSQL(tenantID, viewPrivateTags, section, pick, seed, taken, asOf)
	var ids []int64
	if err := trx.Scalar(pq.Array(&ids), "SELECT ARRAY(SELECT p.id "+statement+")", args...); err != nil {
		return nil, errors.Wrap(err, "failed to select feedback export posts")
	}
	return ids, nil
}

// Pool matches must fit in a PostgreSQL bigint under the recipe limits.
const _ int64 = 1 << (entity.MaxFeedbackExportSections*entity.MaxFeedbackExportPicks - 1)

func selectRandomFeedbackExportPools(trx *dbx.Trx, tenantID int, viewPrivateTags bool, pools []*feedbackExportPool, seed string, asOf time.Time) (err error) {
	args := []any{tenantID, seed}
	bind := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	matchExpressions := make([]string, len(pools))
	for index, pool := range pools {
		predicate := feedbackExportFilterSQL(pool.Section, pool.Pick, viewPrivateTags, asOf, bind)
		matchExpressions[index] = "CASE WHEN " + predicate + " THEN " + bind(int64(1)<<index) + "::bigint ELSE 0 END"
		pool.IDs = make([]int64, 0, pool.FirstRow+pool.Count)
	}

	// One shared scan avoids sorting the same posts for each random pick.
	cursor := pq.QuoteIdentifier("feedback_export_" + rand.String(16))
	statement := "DECLARE " + cursor + ` NO SCROLL CURSOR FOR
		SELECT p.id, (` + strings.Join(matchExpressions, " | ") + `) AS pool_mask
		FROM posts p
		WHERE p.tenant_id = $1 AND p.status IN (0, 1, 2, 3, 4, 5) AND NOT p.moderation_pending
		ORDER BY decode(md5($2 || p.id::text), 'hex'), p.id DESC`

	if _, err = trx.Execute(statement, args...); err != nil {
		return errors.Wrap(err, "failed to order feedback export candidates")
	}

	defer func() {
		if _, closeErr := trx.Execute("CLOSE " + cursor); err == nil && closeErr != nil {
			err = errors.Wrap(closeErr, "failed to close feedback export candidates")
		}
	}()

	remaining := len(pools)
	for remaining > 0 {
		var candidates []*struct {
			ID       int64 `db:"id"`
			PoolMask int64 `db:"pool_mask"`
		}
		if err = trx.Select(&candidates, "FETCH FORWARD 512 FROM "+cursor); err != nil {
			return errors.Wrap(err, "failed to read feedback export candidates")
		}

		for _, candidate := range candidates {
			matchingPools := uint64(candidate.PoolMask)
			for matchingPools != 0 {
				index := bits.TrailingZeros64(matchingPools)
				matchingPools &= matchingPools - 1

				pool := pools[index]
				limit := pool.FirstRow + pool.Count
				if len(pool.IDs) == limit {
					continue
				}

				pool.IDs = append(pool.IDs, candidate.ID)
				if len(pool.IDs) == limit {
					remaining--
				}
			}
		}

		if len(candidates) < 512 {
			break
		}
	}
	return nil
}
