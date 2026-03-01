package db

import (
	"time"

	"gorm.io/gorm"
)

type WebVital struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Service   string         `gorm:"index;not null" json:"service"`
	Name      string         `gorm:"index;not null" json:"name"`
	Value     float64        `gorm:"not null" json:"value"`
	Delta     float64        `json:"delta"`
	VitalID   string         `json:"vital_id"`
	Label     string         `json:"label"`
}

type ReportToEntry struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	CreatedAt          time.Time      `gorm:"index" json:"created_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	Service            string         `gorm:"index;not null" json:"service"`
	ReportType         string         `gorm:"index" json:"report_type"`
	DocumentURI        string         `json:"document_uri"`
	BlockedURI         string         `json:"blocked_uri"`
	ViolatedDirective  string         `json:"violated_directive"`
	EffectiveDirective string         `json:"effective_directive"`
	OriginalPolicy     string         `json:"original_policy"`
	SourceFile         string         `json:"source_file"`
	LineNumber         int            `json:"line_number"`
	ColumnNumber       int            `json:"column_number"`
	StatusCode         int            `json:"status_code"`
	RawJSON            string         `gorm:"type:jsonb" json:"raw_json,omitempty"`
}

type SecurityReportEntry struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	CreatedAt          time.Time      `gorm:"index" json:"created_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	Service            string         `gorm:"index;not null" json:"service"`
	ReportType         string         `gorm:"index;not null" json:"report_type"`
	URL                string         `json:"url"`
	DocumentURI        string         `json:"document_uri"`
	BlockedURI         string         `json:"blocked_uri"`
	ViolatedDirective  string         `json:"violated_directive"`
	EffectiveDirective string         `json:"effective_directive"`
	SourceFile         string         `json:"source_file"`
	LineNumber         int            `json:"line_number"`
	ColumnNumber       int            `json:"column_number"`
	Message            string         `json:"message"`
	RawJSON            string         `gorm:"type:jsonb" json:"raw_json,omitempty"`
}
