package lib

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/icco/gutil/logging"
	"google.golang.org/api/iterator"
)

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
)

// GetServices returns the list of services present in the table.
func GetServices(ctx context.Context, project, dataset, atable, rtable string) ([]string, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("connecting to bq: %w", err)
	}

	var ret []string
	seen := map[string]bool{}

	for _, table := range []string{atable, rtable} {
		t := client.Dataset(dataset).Table(table)
		tableID, err := t.Identifier(bigquery.StandardSQLID)
		if err != nil {
			return nil, fmt.Errorf("getting table id: %w", err)
		}

		q := client.Query(fmt.Sprintf("SELECT DISTINCT Service FROM `%s` WHERE Service IS NOT NULL;", tableID))
		log.Debugw("query prepped", "query", q)
		it, err := q.Read(ctx)
		if err != nil {
			return nil, err
		}

		for {
			var row []bigquery.Value
			err := it.Next(&row)
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("couldn't get Services: %w", err)
			}

			s := row[0].(string)
			if !seen[s] {
				ret = append(ret, s)
				seen[s] = true
			}
		}
	}

	return ret, nil
}
