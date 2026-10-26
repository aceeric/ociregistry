package subcmd

import (
	"fmt"
	"strings"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/serialize"

	log "github.com/sirupsen/logrus"
)

// NotMigratedMsg is the error shown by the load, list and prune commands when the cache has
// legacy manifests that have not been migrated.
const NotMigratedMsg = "You must manually migrate the cache using the migrate-cache command, or run the serve command which automatically migrates the cache"

// MigrateCache migrates the cache from the legacy layout to the current layout and prints the
// report to the console. With --dry-run nothing is written. An error is returned if the
// migration is incomplete (a manifest could not be written); skipped files are not errors
// and are listed in the report.
func MigrateCache() error {
	report, err := serialize.Migrate(config.GetImagePath(), config.GetMigrateConfig().DryRun)
	fmt.Print(report.String())
	return err
}

// migrateOnStartup migrates the cache when the server starts, logging the summary. It returns
// an error, which stops the server from starting, if the migration is incomplete.
func migrateOnStartup(imagePath string) error {
	report, err := serialize.Migrate(imagePath, false)
	if report.Found > 0 {
		for line := range strings.SplitSeq(strings.TrimSpace(report.Summary()), "\n") {
			log.Info(line)
		}
	}
	if err != nil {
		return fmt.Errorf("the cache could not be migrated, so the server will not start: %w", err)
	}
	return nil
}
