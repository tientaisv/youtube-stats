package models

import (
	"time"
)

// Channel represents a YouTube channel being tracked
type Channel struct {
	ID                string    `json:"id" db:"id"`
	ChannelID         string    `json:"channel_id" db:"channel_id"`
	Handle            string    `json:"handle" db:"handle"`
	Title             string    `json:"title" db:"title"`
	UploadsPlaylistID string    `json:"uploads_playlist_id" db:"uploads_playlist_id"`
	ThumbnailURL      string    `json:"thumbnail_url" db:"thumbnail_url"`
	IsMyChannel       bool      `json:"is_my_channel" db:"is_my_channel"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// Video represents a YouTube video
type Video struct {
	ID              string    `json:"id" db:"id"`
	ChannelID       string    `json:"channel_id" db:"channel_id"`
	VideoID         string    `json:"video_id" db:"video_id"`
	Title           string    `json:"title" db:"title"`
	Description     string    `json:"description" db:"description"`
	Tags            string    `json:"tags" db:"tags"` // JSON string array
	PublishedAt     time.Time `json:"published_at" db:"published_at"`
	DurationSeconds int       `json:"duration_seconds" db:"duration_seconds"`
	ThumbnailURL    string    `json:"thumbnail_url" db:"thumbnail_url"`
	IsDeleted       bool      `json:"is_deleted" db:"is_deleted"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`

	// Latest snapshot metrics (joined for video list view)
	LatestViews    int64     `json:"latest_views,omitempty" db:"latest_views"`
	LatestLikes    int64     `json:"latest_likes,omitempty" db:"latest_likes"`
	LatestComments int64     `json:"latest_comments,omitempty" db:"latest_comments"`
	LatestCollectedAt *time.Time `json:"latest_collected_at,omitempty" db:"latest_collected_at"`
}

// VideoSnapshot represents historical metrics snapshot for a video
type VideoSnapshot struct {
	ID           int64     `json:"id" db:"id"`
	VideoID      string    `json:"video_id" db:"video_id"`
	CollectedAt  time.Time `json:"collected_at" db:"collected_at"`
	ViewCount    int64     `json:"view_count" db:"view_count"`
	LikeCount    int64     `json:"like_count" db:"like_count"`
	CommentCount int64     `json:"comment_count" db:"comment_count"`
}

// ChannelSnapshot represents historical metrics snapshot for a channel
type ChannelSnapshot struct {
	ID              int64     `json:"id" db:"id"`
	ChannelID       string    `json:"channel_id" db:"channel_id"`
	CollectedAt     time.Time `json:"collected_at" db:"collected_at"`
	SubscriberCount int64     `json:"subscriber_count" db:"subscriber_count"`
	TotalViewCount  int64     `json:"total_view_count" db:"total_view_count"`
	VideoCount      int64     `json:"video_count" db:"video_count"`
}

// APIKeyRecord represents a Google API Key in rotation pool
type APIKeyRecord struct {
	ID               int        `json:"id" db:"id"`
	KeyValue         string     `json:"key_value" db:"key_value"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	DailyQuotaUsed   int        `json:"daily_quota_used" db:"daily_quota_used"`
	QuotaExceededAt *time.Time `json:"quota_exceeded_at,omitempty" db:"quota_exceeded_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

// HourlyPeakStat represents view gain per hour of day (0-23)
type HourlyPeakStat struct {
	Hour           int    `json:"hour"`            // 0 - 23
	HourFormatted  string `json:"hour_formatted"`  // "08:00 - 09:00"
	TotalViewDelta int64  `json:"total_view_delta"` // Total views gained in this hour slot
	AvgViewDelta   int64  `json:"avg_view_delta"`   // Average views gained per snapshot in this hour
}

// VideoPageResult represents a paginated list of videos
type VideoPageResult struct {
	Items      []Video `json:"items"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	TotalPages int     `json:"total_pages"`
}

// CompetitorMetric holds calculated comparison stats for a channel
type CompetitorMetric struct {
	ChannelID        string  `json:"channel_id"`
	Title            string  `json:"title"`
	Handle           string  `json:"handle"`
	ThumbnailURL     string  `json:"thumbnail_url"`
	IsMyChannel      bool    `json:"is_my_channel"`
	SubscriberCount  int64   `json:"subscriber_count"`
	TotalViewCount   int64   `json:"total_view_count"`
	VideoCount       int64   `json:"video_count"`
	AvgViewsPerVideo int64   `json:"avg_views_per_video"`
	EngagementRate   float64 `json:"engagement_rate"`   // (Likes + Comments) / Views * 100
	PostingFrequency float64 `json:"posting_frequency"` // Videos per week in last 30 days
}

// MultiLinePoint represents snapshot comparison data at a point in time
type MultiLinePoint struct {
	CollectedAt time.Time        `json:"collected_at"`
	Values      map[string]int64 `json:"values"` // ChannelID -> ViewCount
}

// CompetitorCompareResponse is the full comparison payload
type CompetitorCompareResponse struct {
	MyChannel      *CompetitorMetric `json:"my_channel"`
	Competitors    []CompetitorMetric `json:"competitors"`
	GrowthTimeline []MultiLinePoint   `json:"growth_timeline"`
}

// User represents a system user account
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"` // "admin" or "user"
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// UserInfo is a safe representation of user data without sensitive hashes
type UserInfo struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// UserSession represents an active user session token
type UserSession struct {
	Token     string    `json:"token" db:"token"`
	UserID    int64     `json:"user_id" db:"user_id"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// LoginRequest DTO
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateUserRequest DTO
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// DashboardStats provides overview metrics for the web dashboard
type DashboardStats struct {
	TotalChannels    int64     `json:"total_channels"`
	TotalVideos      int64     `json:"total_videos"`
	TotalViews       int64     `json:"total_views"`
	TotalSnapshots   int64     `json:"total_snapshots"`
	ActiveAPIKeys    int       `json:"active_api_keys"`
	LastCollectedAt *time.Time `json:"last_collected_at,omitempty"`
	NextScheduleAt   *time.Time `json:"next_schedule_at,omitempty"`
	CronSchedule     string    `json:"cron_schedule"`
}
