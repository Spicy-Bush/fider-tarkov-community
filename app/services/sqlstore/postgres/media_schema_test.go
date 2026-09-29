package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres/mediaowners"
)

func readSchemaDefinitions(trx *sql.Tx) (map[string]string, error) {
	rows, err := trx.Query(`
		SELECT 'function ' || p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')' AS name,
			pg_get_functiondef(p.oid) AS definition
		FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
		WHERE n.nspname=current_schema() AND p.prokind='f'
		UNION ALL
		SELECT 'view ' || c.relname, pg_get_viewdef(c.oid)
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname=current_schema() AND c.relkind='v'
		UNION ALL
		SELECT 'trigger ' || c.relname || '.' || t.tgname,
			pg_get_triggerdef(t.oid) || ' enabled=' || t.tgenabled
		FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid
		JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname=current_schema() AND NOT t.tgisinternal
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	definitions := make(map[string]string)
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			return nil, err
		}
		definitions[name] = definition
	}

	return definitions, rows.Err()
}

func mediaSchemaDifferences(trx *sql.Tx) (differences []string, err error) {
	before, err := readSchemaDefinitions(trx)
	if err != nil {
		return nil, err
	}

	// PostgreSQL normalises both definitions; the savepoint leaves the migrated schema intact.
	if _, err := trx.Exec("SAVEPOINT media_schema_check"); err != nil {
		return nil, err
	}
	defer func() {
		_, cleanupErr := trx.Exec("ROLLBACK TO SAVEPOINT media_schema_check; RELEASE SAVEPOINT media_schema_check")
		err = errors.Join(err, cleanupErr)
	}()

	if _, err := trx.Exec(mediaowners.UpdateSQL()); err != nil {
		return nil, fmt.Errorf("current media definitions do not fit the migrated database: %w", err)
	}
	after, err := readSchemaDefinitions(trx)
	if err != nil {
		return nil, err
	}

	for name, definition := range after {
		if before[name] != definition {
			differences = append(differences, name)
		}
	}

	owners := make(map[string]bool, len(mediaowners.All))
	for _, owner := range mediaowners.All {
		owners["function capture_media_"+owner.Kind+"()"] = true
	}
	for name := range before {
		if strings.HasPrefix(name, "function capture_media_") && !owners[name] {
			differences = append(differences, name)
		}
	}

	slices.Sort(differences)
	return differences, nil
}

func TestMediaSchemaMatchesMigratedDatabase(t *testing.T) {
	trx, err := dbx.Connection().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.Rollback()

	differences, err := mediaSchemaDifferences(trx)
	if err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("add a migration for the changed media definitions: %v", differences)
	}
}

func TestMediaSchemaDetectsDriftWithoutRepairingIt(t *testing.T) {
	for _, test := range []struct {
		name      string
		statement string
		object    string
	}{
		{
			name:      "missing trigger",
			statement: "DROP TRIGGER media_post_references ON posts",
			object:    "trigger posts.media_post_references",
		},
		{
			name:      "disabled trigger",
			statement: "ALTER TABLE posts DISABLE TRIGGER media_post_references",
			object:    "trigger posts.media_post_references",
		},
		{
			name: "incomplete capture function",
			statement: `CREATE OR REPLACE FUNCTION capture_media_post() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN RETURN NULL; END;
			$$`,
			object: "function capture_media_post()",
		},
		{
			name: "stale usage link",
			statement: `CREATE OR REPLACE VIEW media_reference_sources AS
				SELECT tenant_id, 'post'::text AS kind, id AS owner_id,
					title::varchar, '/incorrect'::text AS url, '{}'::jsonb AS source FROM posts`,
			object: "view media_reference_sources",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			trx, err := dbx.Connection().BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer trx.Rollback()

			if _, err := trx.Exec(test.statement); err != nil {
				t.Fatal(err)
			}
			before, err := readSchemaDefinitions(trx)
			if err != nil {
				t.Fatal(err)
			}

			differences, err := mediaSchemaDifferences(trx)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(differences, test.object) {
				t.Fatalf("missed %s: %v", test.name, differences)
			}

			after, err := readSchemaDefinitions(trx)
			if err != nil {
				t.Fatal(err)
			}
			if before[test.object] != after[test.object] {
				t.Fatal("schema validation repaired the defect it should report")
			}
		})
	}
}

func TestMediaSchemaAcceptsLaterMigration(t *testing.T) {
	trx, err := dbx.Connection().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.Rollback()

	previous := mediaowners.All
	mediaowners.All = slices.Clone(previous)
	t.Cleanup(func() { mediaowners.All = previous })
	for index, owner := range mediaowners.All {
		if owner.Kind == "post" {
			mediaowners.All[index].Keys = append(slices.Clone(owner.Keys), "schema_fixture_image")
		}
	}

	if _, err := mediaSchemaDifferences(trx); err == nil {
		t.Fatal("missing image column was accepted")
	}

	if _, err := trx.Exec("ALTER TABLE posts ADD COLUMN schema_fixture_image text NOT NULL DEFAULT ''"); err != nil {
		t.Fatal(err)
	}
	differences, err := mediaSchemaDifferences(trx)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range []string{"function capture_media_post()", "trigger posts.media_post_references", "view media_reference_sources"} {
		if !slices.Contains(differences, object) {
			t.Fatalf("new image field did not require updating %s: %v", object, differences)
		}
	}

	if _, err := trx.Exec(mediaowners.UpdateSQL()); err != nil {
		t.Fatal(err)
	}
	differences, err = mediaSchemaDifferences(trx)
	if err != nil || len(differences) != 0 {
		t.Fatalf("later migration did not satisfy the current definitions: differences=%v error=%v", differences, err)
	}

	var postID int
	if err := trx.QueryRow(`INSERT INTO posts (tenant_id,user_id,title,slug,number,status,created_at,description)
		VALUES (1,1,'Schema fixture','schema-fixture',10001,0,NOW(),'') RETURNING id`).Scan(&postID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Exec("UPDATE posts SET schema_fixture_image='assets/later-migration' WHERE id=$1", postID); err != nil {
		t.Fatal(err)
	}
	var captured string
	if err := trx.QueryRow(`SELECT source->>'schema_fixture_image' FROM media_reference_changes
		WHERE transaction_id=txid_current() AND kind='post' AND owner_id=$1`, postID).Scan(&captured); err != nil {
		t.Fatal(err)
	}
	if captured != "assets/later-migration" {
		t.Fatalf("new field was not captured: %q", captured)
	}
}

func TestMediaSchemaRequiresRemovedOwnerCleanup(t *testing.T) {
	trx, err := dbx.Connection().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.Rollback()

	previous := mediaowners.All
	mediaowners.All = slices.DeleteFunc(slices.Clone(previous), func(owner mediaowners.Owner) bool {
		return owner.Kind == "webhook"
	})
	t.Cleanup(func() { mediaowners.All = previous })

	differences, err := mediaSchemaDifferences(trx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(differences, "function capture_media_webhook()") {
		t.Fatalf("obsolete owner was accepted: %v", differences)
	}

	if _, err := trx.Exec(`DROP TRIGGER media_webhook_references ON webhooks;
		DROP FUNCTION capture_media_webhook();`); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Exec(mediaowners.UpdateSQL()); err != nil {
		t.Fatal(err)
	}

	differences, err = mediaSchemaDifferences(trx)
	if err != nil || len(differences) != 0 {
		t.Fatalf("owner cleanup did not satisfy the current definitions: differences=%v error=%v", differences, err)
	}
}
