package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/icco/reportd/pkg/analytics"
	"github.com/icco/reportd/pkg/db"
	"github.com/icco/reportd/pkg/reporting"
	"github.com/icco/reportd/pkg/reportto"
	"github.com/namsral/flag"
	"google.golang.org/api/iterator"
	"gorm.io/gorm"
)

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "REPORTD", 0)
	project := fs.String("project", "", "GCP project ID")
	dataset := fs.String("dataset", "", "BigQuery dataset")
	aTable := fs.String("analytics_table", "", "BQ analytics table name")
	rTable := fs.String("reports_table", "", "BQ reports table name")
	rv2Table := fs.String("reports_v2_table", "", "BQ reporting (v2) table name")
	databaseURL := fs.String("database_url", "", "Database connection string")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("parsing flags: %v", err)
	}

	for _, kv := range []struct{ name, val string }{
		{"project", *project},
		{"dataset", *dataset},
		{"analytics_table", *aTable},
		{"reports_table", *rTable},
		{"reports_v2_table", *rv2Table},
		{"database_url", *databaseURL},
	} {
		if kv.val == "" {
			log.Fatalf("%s is required", kv.name)
		}
	}

	ctx := context.Background()

	pgDB, err := db.Connect(ctx, *databaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	if err := db.AutoMigrate(ctx, pgDB); err != nil {
		log.Fatalf("auto-migrating: %v", err)
	}

	bqClient, err := bigquery.NewClient(ctx, *project)
	if err != nil {
		log.Fatalf("connecting to bigquery: %v", err)
	}
	defer func() { _ = bqClient.Close() }()

	// Clean up bad data from previous shell-script migration.
	log.Println("==> Cleaning up bad migration data...")
	for _, table := range []string{"web_vitals", "report_to_entries", "security_report_entries"} {
		res := pgDB.Exec(fmt.Sprintf(`DELETE FROM %s WHERE service ~ '^\d{4}-\d{2}-\d{2}' OR service = ''`, table))
		if res.Error != nil {
			log.Printf("    warning: cleaning %s: %v", table, res.Error)
		} else {
			log.Printf("    %s: deleted %d bad rows", table, res.RowsAffected) //nolint:gosec // table is from a hardcoded list, not user input
		}
	}

	log.Println("")
	migrateAnalytics(ctx, bqClient, pgDB, *project, *dataset, *aTable)
	log.Println("")
	migrateReports(ctx, bqClient, pgDB, *project, *dataset, *rTable)
	log.Println("")
	migrateReporting(ctx, bqClient, pgDB, *project, *dataset, *rv2Table)

	log.Println("")
	log.Println("==> Migration complete.")
}

func migrateAnalytics(ctx context.Context, bq *bigquery.Client, pgDB *gorm.DB, project, dataset, table string) {
	log.Println("==> Migrating analytics (web vitals)...")

	q := bq.Query(fmt.Sprintf("SELECT * FROM `%s.%s.%s` ORDER BY Time ASC", project, dataset, table))
	it, err := q.Read(ctx)
	if err != nil {
		log.Fatalf("querying analytics: %v", err)
	}

	var batch []*db.WebVital
	total := 0
	skipped := 0

	for {
		var row analytics.WebVital
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Fatalf("reading analytics row: %v", err)
		}

		if !row.Service.Valid || row.Service.StringVal == "" {
			skipped++
			continue
		}

		createdAt := time.Now()
		if row.Time.Valid {
			createdAt = row.Time.DateTime.In(time.UTC)
		}

		batch = append(batch, &db.WebVital{
			CreatedAt: createdAt,
			Service:   row.Service.StringVal,
			Name:      row.Name,
			Value:     row.Value,
			Delta:     row.Delta,
			VitalID:   row.ID,
			Label:     row.Label.StringVal,
		})

		if len(batch) >= 500 {
			if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
				log.Fatalf("inserting analytics batch: %v", err)
			}
			total += len(batch)
			log.Printf("    inserted %d rows so far...", total)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
			log.Fatalf("inserting analytics batch: %v", err)
		}
		total += len(batch)
	}

	log.Printf("    Done: %d web_vitals rows", total)
	if skipped > 0 {
		log.Printf("    WARNING: skipped %d rows with empty service", skipped)
	}
}

func migrateReports(ctx context.Context, bq *bigquery.Client, pgDB *gorm.DB, project, dataset, table string) {
	log.Println("==> Migrating reports (report-to entries)...")

	q := bq.Query(fmt.Sprintf("SELECT * FROM `%s.%s.%s` ORDER BY Time ASC", project, dataset, table))
	it, err := q.Read(ctx)
	if err != nil {
		log.Fatalf("querying reports: %v", err)
	}

	var batch []*db.ReportToEntry
	total := 0
	skipped := 0

	for {
		var row reportto.Report
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Fatalf("reading reports row: %v", err)
		}

		if !row.Service.Valid || row.Service.StringVal == "" {
			skipped++
			continue
		}

		createdAt := time.Now()
		if row.Time.Valid {
			createdAt = row.Time.DateTime.In(time.UTC)
		}
		srv := row.Service.StringVal

		// Reuse the same conversion logic as the live handler.
		entries := db.ReportToEntriesFromReport(&row)
		for _, e := range entries {
			e.CreatedAt = createdAt
			e.Service = srv
		}

		batch = append(batch, entries...)

		if len(batch) >= 500 {
			if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
				log.Fatalf("inserting reports batch: %v", err)
			}
			total += len(batch)
			log.Printf("    inserted %d rows so far...", total)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
			log.Fatalf("inserting reports batch: %v", err)
		}
		total += len(batch)
	}

	log.Printf("    Done: %d report_to_entries rows", total)
	if skipped > 0 {
		log.Printf("    WARNING: skipped %d rows with empty service", skipped)
	}
}

func migrateReporting(ctx context.Context, bq *bigquery.Client, pgDB *gorm.DB, project, dataset, table string) {
	log.Println("==> Migrating reporting (security reports v2)...")

	q := bq.Query(fmt.Sprintf("SELECT * FROM `%s.%s.%s` ORDER BY Time ASC", project, dataset, table))
	it, err := q.Read(ctx)
	if err != nil {
		log.Fatalf("querying reporting: %v", err)
	}

	var batch []*db.SecurityReportEntry
	total := 0
	skipped := 0

	for {
		var row reporting.SecurityReport
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Fatalf("reading reporting row: %v", err)
		}

		if !row.Service.Valid || row.Service.StringVal == "" {
			skipped++
			continue
		}

		createdAt := time.Now()
		if row.Time.Valid {
			createdAt = row.Time.DateTime.In(time.UTC)
		}

		// Reconstruct RawJSON since it's tagged bigquery:"-".
		if row.RawJSON == "" {
			if raw, err := json.Marshal(row); err == nil {
				row.RawJSON = string(raw)
			}
		}

		// Reuse the same conversion logic as the live handler.
		entry := db.SecurityReportEntryFromReport(&row)
		entry.CreatedAt = createdAt

		batch = append(batch, entry)

		if len(batch) >= 500 {
			if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
				log.Fatalf("inserting reporting batch: %v", err)
			}
			total += len(batch)
			log.Printf("    inserted %d rows so far...", total)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := pgDB.CreateInBatches(batch, 500).Error; err != nil {
			log.Fatalf("inserting reporting batch: %v", err)
		}
		total += len(batch)
	}

	log.Printf("    Done: %d security_report_entries rows", total)
	if skipped > 0 {
		log.Printf("    WARNING: skipped %d rows with empty service", skipped)
	}
}
