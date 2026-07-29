package collector

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"youtube-manager/internal/database"
	"youtube-manager/internal/models"
	"youtube-manager/internal/youtube"
)

type Collector struct {
	db *database.DB
	yt *youtube.Client
}

func NewCollector(db *database.DB, yt *youtube.Client) *Collector {
	return &Collector{db: db, yt: yt}
}

// SyncChannel collects channel metadata and all its videos & snapshots
func (c *Collector) SyncChannel(channelID string) error {
	chanInfo, err := c.yt.ResolveChannel(channelID)
	if err != nil {
		return fmt.Errorf("failed resolving channel %s: %w", channelID, err)
	}

	now := time.Now()

	// 1. Save or update channel
	chanModel := &models.Channel{
		ID:                chanInfo.ChannelID,
		ChannelID:         chanInfo.ChannelID,
		Handle:            chanInfo.Handle,
		Title:             chanInfo.Title,
		UploadsPlaylistID: chanInfo.UploadsPlaylistID,
		ThumbnailURL:      chanInfo.ThumbnailURL,
		CreatedAt:         now,
	}
	if err := c.db.SaveChannel(chanModel); err != nil {
		return fmt.Errorf("failed saving channel %s: %w", chanInfo.ChannelID, err)
	}

	// 2. Record channel snapshot
	cSnap := &models.ChannelSnapshot{
		ChannelID:       chanInfo.ChannelID,
		CollectedAt:     now,
		SubscriberCount: chanInfo.SubscriberCount,
		TotalViewCount:  chanInfo.TotalViewCount,
		VideoCount:      chanInfo.VideoCount,
	}
	c.db.SaveChannelSnapshot(cSnap)

	// 3. Fetch playlist videos
	if chanInfo.UploadsPlaylistID == "" {
		return nil
	}

	videoIDs, err := c.yt.GetPlaylistVideos(chanInfo.UploadsPlaylistID)
	if err != nil {
		return fmt.Errorf("failed getting playlist videos: %w", err)
	}

	if len(videoIDs) == 0 {
		return nil
	}

	// 4. Batch get video details & create snapshots
	vDetails, err := c.yt.BatchGetVideoDetails(videoIDs)
	if err != nil {
		return fmt.Errorf("failed batch fetching video details: %w", err)
	}

	for _, vd := range vDetails {
		tagsJSON, _ := json.Marshal(vd.Tags)

		vModel := &models.Video{
			ID:              vd.VideoID,
			ChannelID:       chanInfo.ChannelID,
			VideoID:         vd.VideoID,
			Title:           vd.Title,
			Description:     vd.Description,
			Tags:            string(tagsJSON),
			PublishedAt:     vd.PublishedAt,
			DurationSeconds: vd.DurationSeconds,
			ThumbnailURL:    vd.ThumbnailURL,
			IsDeleted:       false,
			CreatedAt:       now,
		}

		if err := c.db.SaveVideo(vModel); err != nil {
			log.Printf("[ERROR] failed saving video %s: %v", vd.VideoID, err)
			continue
		}

		vSnap := &models.VideoSnapshot{
			VideoID:      vd.VideoID,
			CollectedAt:  now,
			ViewCount:    vd.ViewCount,
			LikeCount:    vd.LikeCount,
			CommentCount: vd.CommentCount,
		}
		c.db.SaveVideoSnapshot(vSnap)
	}

	log.Printf("[SUCCESS] Synced channel %s (%s): %d videos updated with snapshots", chanInfo.Title, chanInfo.ChannelID, len(vDetails))
	return nil
}

// CollectAll syncs all channels saved in DB
func (c *Collector) CollectAll() error {
	channels, err := c.db.GetChannels()
	if err != nil {
		return fmt.Errorf("failed fetching channels from DB: %w", err)
	}

	if len(channels) == 0 {
		log.Println("[INFO] No channels configured for collection.")
		return nil
	}

	log.Printf("[INFO] Starting periodic data collection for %d channels...", len(channels))
	for _, ch := range channels {
		if err := c.SyncChannel(ch.ChannelID); err != nil {
			log.Printf("[ERROR] Failed syncing channel %s: %v", ch.ChannelID, err)
		}
	}
	log.Println("[INFO] Periodic collection completed successfully.")
	return nil
}
