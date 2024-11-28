package lib

import "github.com/icco/gutil/logging"

var (
	service = "reportd"
	log     = logging.Must(logging.NewLogger(service))
)
