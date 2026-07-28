// Package store writes into the anime-api-owned MySQL tables (anime_news,
// anime_fanart). anime-api creates/migrates these tables; we only upsert rows.
package store

import (
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
}

func (AnimeNews) TableName() string { return "anime_news" }

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
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=%s&interpolateParams=true",
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
func (s *Store) UpsertNews(n *AnimeNews) error {
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"anime_id", "mal_id", "title", "summary", "category", "source_url",
			"source_name", "published_date", "episode_number", "title_slug", "researched_at",
		}),
	}).Create(n).Error
}

// UpsertFanart is insert-if-new (image URL is immutable, nothing to update).
func (s *Store) UpsertFanart(f *Fanart) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(f).Error
}
