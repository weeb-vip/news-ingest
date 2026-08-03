// Package store reads and writes the news tables (anime_news, anime_fanart).
//
// This service OWNS that schema: the migrations live in ../../db/migrations and are applied
// by `news-ingest migrate`. They used to belong to anime-api, which meant the only writer
// did not control the columns it wrote — a deploy landing ahead of anime-api's migration
// silently destroyed a day of messages against a column that did not exist yet.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/weeb-vip/news-ingest/config"
)

type AnimeNews struct {
	ID            string     `gorm:"column:id;primaryKey"`
	AnimeID       string     `gorm:"column:anime_id"`
	MalID         *int       `gorm:"column:mal_id"`
	Title         string     `gorm:"column:title"`
	Summary       *string    `gorm:"column:summary"`
	Category      string     `gorm:"column:category"`
	SourceURL     *string    `gorm:"column:source_url"`
	SourceName    *string    `gorm:"column:source_name"`
	PublishedDate *time.Time `gorm:"column:published_date"`
	EpisodeNumber *int       `gorm:"column:episode_number"`
	TitleSlug     *string    `gorm:"column:title_slug"`
	ResearchedAt  *time.Time `gorm:"column:researched_at"`
	Language      *string    `gorm:"column:language"`
	// MySQL JSON column carried as a marshalled string: an array of
	// {kind,title,url}. nil when the article referenced nothing.
	References *string `gorm:"column:reference_links"`
}

func (AnimeNews) TableName() string { return "anime_news" }

// DB exposes the underlying *sql.DB for the migration runner, which needs a raw handle.
func (s *Store) DB() (*sql.DB, error) { return s.db.DB() }

// Begin returns a Store bound to a new transaction, plus a function that rolls it back.
//
// Tests use this so each one sees a clean database without deleting anything: the writes
// happen, are read back, and then vanish. That is both faster than truncating between
// tests and safer — a DELETE in a test suite is one bad connection string away from
// emptying a real table.
func (s *Store) Begin() (*Store, func()) {
	tx := s.db.Begin()
	return &Store{db: tx}, func() { tx.Rollback() }
}

// Raw runs a query for callers outside this package (the migrator's existence check).
func (s *Store) Raw(query string, args ...any) *gorm.DB { return s.db.Raw(query, args...) }

type Fanart struct {
	ID        string  `gorm:"column:id;primaryKey"`
	AnimeID   string  `gorm:"column:anime_id"`
	ImageURL  string  `gorm:"column:image_url"`
	SourceURL *string `gorm:"column:source_url"`
}

func (Fanart) TableName() string { return "anime_fanart" }

type Store struct{ db *gorm.DB }

func Open(cfg config.DBConfig) (*Store, error) {
	dsn := fmt.Sprintf(
		// loc=UTC, deliberately NOT Local. published_date is a DATE — a calendar date, not an
		// instant — and the producer parses it as UTC midnight. With loc=Local the driver
		// converts on the way in, so on any host behind UTC the date is stored a day early:
		// 2026-08-02 becomes 2026-08-01. That is silent, and wrong in the one field the feed
		// is sorted by. UTC in and UTC out means no conversion happens at all.
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC&tls=%s&interpolateParams=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DataBase, cfg.SSLMode)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	return &Store{db: db}, nil
}

// UpsertNews inserts or updates by primary key (the dedupe id), leaving created_at
// untouched so re-publishing an item never rewrites when it was first seen.
// newsUpdatableColumns is every mutable column on AnimeNews — everything except the primary
// key. A column that exists on the struct but is missing here is written on INSERT and then
// never updated again, so a re-publish silently fails to backfill it on rows that already
// exist. That is invisible in normal testing (new rows look correct) and shows up only as
// stale data on old rows, which is why the test derives the expected set by reflection
// rather than trusting this list to be kept in step by hand.
var newsUpdatableColumns = []string{
	"anime_id", "mal_id", "title", "summary", "category", "source_url",
	"source_name", "published_date", "episode_number", "title_slug", "researched_at",
	"language", "reference_links",
}

func (s *Store) UpsertNews(n *AnimeNews) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns(newsUpdatableColumns),
	}).Create(n).Error
}

// UpsertFanart is insert-if-new (image URL is immutable, nothing to update).
func (s *Store) UpsertFanart(f *Fanart) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(f).Error
}
