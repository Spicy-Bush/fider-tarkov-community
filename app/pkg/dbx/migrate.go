package dbx

import (
	"context"
	"database/sql"
	stdErrors "errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
)

var ErrNoChanges = stdErrors.New("nothing to migrate.")

func Migrate(ctx context.Context, path string) error {
	log.Info(ctx, "Running migrations...")

	dirPath := strings.TrimPrefix(path, "/")
	files, err := fs.ReadDir(assets.FS, dirPath)
	if err != nil {
		return errors.Wrap(err, "failed to read dir '%s'", path)
	}

	versions := make([]int, len(files))
	versionFiles := make(map[int]string, len(files))
	for i, file := range files {
		fileName := file.Name()
		parts := strings.Split(fileName, "_")
		if len(parts[0]) != 12 {
			return errors.New("migration file must have exactly 12 chars for version: '%s' is invalid.", fileName)
		}

		versions[i], err = strconv.Atoi(parts[0])
		versionFiles[versions[i]] = fileName
		if err != nil {
			return errors.Wrap(err, "failed to convert '%s' to number", parts[0])
		}
	}
	sort.Ints(versions)

	log.Infof(ctx, "Found total of @{Total} migration files.", dto.Props{
		"Total": len(versions),
	})

	lastVersion, err := getLastMigration()
	if err != nil {
		return errors.Wrap(err, "failed to get last migration record")
	}

	log.Infof(ctx, "Current version is @{Version}", dto.Props{
		"Version": lastVersion,
	})

	totalMigrationsExecuted := 0

	for _, version := range versions {
		if version > lastVersion {
			fileName := versionFiles[version]
			log.Infof(ctx, "Running Version: @{Version} (@{FileName})", dto.Props{
				"Version":  version,
				"FileName": fileName,
			})
			err := runMigration(ctx, version, dirPath, fileName)
			if err != nil {
				return errors.Wrap(err, "failed to run migration '%s'", fileName)
			}
			totalMigrationsExecuted++
		}
	}

	if totalMigrationsExecuted > 0 {
		log.Infof(ctx, "@{Count} migrations have been applied.", dto.Props{
			"Count": totalMigrationsExecuted,
		})
	} else {
		log.Info(ctx, "Migrations are already up to date.")
	}
	return nil
}

func runMigration(ctx context.Context, version int, path, fileName string) error {
	filePath := path + "/" + fileName
	content, err := fs.ReadFile(assets.FS, filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read file '%s'", filePath)
	}

	trx, err := BeginTx(ctx)
	if err != nil {
		return err
	}

	_, err = trx.tx.Exec(string(content))
	if err != nil {
		return err
	}

	_, err = trx.Execute("INSERT INTO migrations_history (version, filename) VALUES ($1, $2)", version, fileName)
	if err != nil {
		return err
	}

	return trx.Commit()
}

func getLastMigration() (int, error) {
	_, err := conn.Exec(`CREATE TABLE IF NOT EXISTS migrations_history (
		version     BIGINT PRIMARY KEY,
		filename    VARCHAR(100) null,
		date	 			TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return 0, err
	}

	var lastVersion sql.NullInt64
	row := conn.QueryRow("SELECT MAX(version) FROM migrations_history LIMIT 1")
	err = row.Scan(&lastVersion)
	if err != nil {
		return 0, err
	}

	if !lastVersion.Valid {
		row := conn.QueryRow("SELECT version FROM schema_migrations LIMIT 1")
		_ = row.Scan(&lastVersion)
	}

	return int(lastVersion.Int64), nil
}
