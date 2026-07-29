package youtube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KeyPool manages a thread-safe pool of Google API Keys with automatic failover rotation
type KeyPool struct {
	mu          sync.Mutex
	keys        []string
	activeIdx   int
	exceededMap map[string]time.Time
}

func NewKeyPool(keys []string) *KeyPool {
	return &KeyPool{
		keys:        keys,
		activeIdx:   0,
		exceededMap: make(map[string]time.Time),
	}
}

func (kp *KeyPool) AddKey(key string) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	for _, k := range kp.keys {
		if k == key {
			return
		}
	}
	kp.keys = append(kp.keys, key)
}

func (kp *KeyPool) GetCurrentKey() (string, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if len(kp.keys) == 0 {
		return "", fmt.Errorf("no YouTube API keys configured")
	}

	// Reset quota reset after 24h
	now := time.Now()
	for k, t := range kp.exceededMap {
		if now.Sub(t) > 24*time.Hour {
			delete(kp.exceededMap, k)
		}
	}

	// Find first non-exceeded key starting from activeIdx
	startIdx := kp.activeIdx
	for i := 0; i < len(kp.keys); i++ {
		idx := (startIdx + i) % len(kp.keys)
		k := kp.keys[idx]
		if _, exceeded := kp.exceededMap[k]; !exceeded {
			kp.activeIdx = idx
			return k, nil
		}
	}

	return "", fmt.Errorf("all configured YouTube API keys have exceeded daily quota limits")
}

func (kp *KeyPool) MarkQuotaExceeded(key string) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	log.Printf("[WARNING] YouTube API Key %s... reached quota limit! Rotating to next key.", maskKey(key))
	kp.exceededMap[key] = time.Now()
	kp.activeIdx = (kp.activeIdx + 1) % len(kp.keys)
}

func (kp *KeyPool) GetActiveCount() int {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	active := 0
	for _, k := range kp.keys {
		if _, exceeded := kp.exceededMap[k]; !exceeded {
			active++
		}
	}
	return active
}

func maskKey(k string) string {
	if len(k) <= 8 {
		return "***"
	}
	return k[:4] + "..." + k[len(k)-4:]
}

type Client struct {
	keyPool    *KeyPool
	httpClient *http.Client
}

func NewClient(keyPool *KeyPool) *Client {
	return &Client{
		keyPool: keyPool,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) executeRequest(rawURL string) ([]byte, error) {
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		key, err := c.keyPool.GetCurrentKey()
		if err != nil {
			return nil, err
		}

		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("key", key)
		u.RawQuery = q.Encode()

		resp, err := c.httpClient.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
			if strings.Contains(string(body), "quotaExceeded") || strings.Contains(string(body), "dailyLimitExceeded") {
				c.keyPool.MarkQuotaExceeded(key)
				continue // retry with next key
			}
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("YouTube API error (status %d): %s", resp.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("failed after rotating API keys")
}

// ChannelInfo DTO
type ChannelInfo struct {
	ChannelID         string
	Handle            string
	Title             string
	UploadsPlaylistID string
	ThumbnailURL      string
	SubscriberCount   int64
	TotalViewCount    int64
	VideoCount        int64
}

// ResolveChannel fetches channel metadata by handle (@name) or channel ID (UC...)
func (c *Client) ResolveChannel(input string) (*ChannelInfo, error) {
	input = strings.TrimSpace(input)
	endpoint := "https://www.googleapis.com/youtube/v3/channels?part=snippet,contentDetails,statistics"

	if strings.HasPrefix(input, "@") {
		endpoint += "&forHandle=" + url.QueryEscape(input)
	} else if strings.HasPrefix(input, "UC") && len(input) >= 20 {
		endpoint += "&id=" + url.QueryEscape(input)
	} else {
		// try forHandle by prepending @
		endpoint += "&forHandle=" + url.QueryEscape("@"+input)
	}

	body, err := c.executeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var res struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				CustomURL  string `json:"customUrl"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
			Statistics struct {
				SubscriberCount string `json:"subscriberCount"`
				ViewCount       string `json:"viewCount"`
				VideoCount      string `json:"videoCount"`
			} `json:"statistics"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	if len(res.Items) == 0 {
		return nil, fmt.Errorf("no YouTube channel found for input: %s", input)
	}

	item := res.Items[0]
	subCount, _ := strconv.ParseInt(item.Statistics.SubscriberCount, 10, 64)
	viewCount, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
	vidCount, _ := strconv.ParseInt(item.Statistics.VideoCount, 10, 64)

	thumb := item.Snippet.Thumbnails.High.URL
	if thumb == "" {
		thumb = item.Snippet.Thumbnails.Default.URL
	}

	return &ChannelInfo{
		ChannelID:         item.ID,
		Handle:            item.Snippet.CustomURL,
		Title:             item.Snippet.Title,
		UploadsPlaylistID: item.ContentDetails.RelatedPlaylists.Uploads,
		ThumbnailURL:      thumb,
		SubscriberCount:   subCount,
		TotalViewCount:    viewCount,
		VideoCount:        vidCount,
	}, nil
}

// GetPlaylistVideos fetches all video IDs in a playlist (e.g. uploads playlist)
func (c *Client) GetPlaylistVideos(playlistID string) ([]string, error) {
	var videoIDs []string
	nextPageToken := ""

	for {
		rawURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/playlistItems?part=contentDetails&maxResults=50&playlistId=%s", url.QueryEscape(playlistID))
		if nextPageToken != "" {
			rawURL += "&pageToken=" + nextPageToken
		}

		body, err := c.executeRequest(rawURL)
		if err != nil {
			return nil, err
		}

		var res struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}

		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}

		for _, item := range res.Items {
			if item.ContentDetails.VideoID != "" {
				videoIDs = append(videoIDs, item.ContentDetails.VideoID)
			}
		}

		if res.NextPageToken == "" {
			break
		}
		nextPageToken = res.NextPageToken
	}

	return videoIDs, nil
}

type VideoDetails struct {
	VideoID         string
	Title           string
	Description     string
	Tags            []string
	PublishedAt     time.Time
	DurationSeconds int
	ThumbnailURL    string
	ViewCount       int64
	LikeCount       int64
	CommentCount    int64
}

// BatchGetVideoDetails fetches details for video IDs in batches of 50
func (c *Client) BatchGetVideoDetails(videoIDs []string) ([]VideoDetails, error) {
	var results []VideoDetails

	batchSize := 50
	for i := 0; i < len(videoIDs); i += batchSize {
		end := i + batchSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		chunk := videoIDs[i:end]
		idList := strings.Join(chunk, ",")

		rawURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=snippet,statistics,contentDetails&id=%s", url.QueryEscape(idList))
		body, err := c.executeRequest(rawURL)
		if err != nil {
			return nil, err
		}

		var res struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					PublishedAt string   `json:"publishedAt"`
					Title       string   `json:"title"`
					Description string   `json:"description"`
					Tags        []string `json:"tags"`
					Thumbnails  struct {
						High struct {
							URL string `json:"url"`
						} `json:"high"`
						Default struct {
							URL string `json:"url"`
						} `json:"default"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Statistics struct {
					ViewCount    string `json:"viewCount"`
					LikeCount    string `json:"likeCount"`
					CommentCount string `json:"commentCount"`
				} `json:"statistics"`
			} `json:"items"`
		}

		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}

		for _, item := range res.Items {
			pubTime, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			durationSecs := ParseISO8601Duration(item.ContentDetails.Duration)
			vCount, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
			lCount, _ := strconv.ParseInt(item.Statistics.LikeCount, 10, 64)
			cCount, _ := strconv.ParseInt(item.Statistics.CommentCount, 10, 64)

			thumb := item.Snippet.Thumbnails.High.URL
			if thumb == "" {
				thumb = item.Snippet.Thumbnails.Default.URL
			}

			results = append(results, VideoDetails{
				VideoID:         item.ID,
				Title:           item.Snippet.Title,
				Description:     item.Snippet.Description,
				Tags:            item.Snippet.Tags,
				PublishedAt:     pubTime,
				DurationSeconds: durationSecs,
				ThumbnailURL:    thumb,
				ViewCount:       vCount,
				LikeCount:       lCount,
				CommentCount:    cCount,
			})
		}
	}

	return results, nil
}

// ParseISO8601Duration converts ISO 8601 duration strings (e.g. PT15M33S, PT1H2M5S) to seconds
func ParseISO8601Duration(isoDuration string) int {
	re := regexp.MustCompile(`P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)
	matches := re.FindStringSubmatch(isoDuration)
	if len(matches) == 0 {
		return 0
	}

	days, _ := strconv.Atoi(matches[1])
	hours, _ := strconv.Atoi(matches[2])
	minutes, _ := strconv.Atoi(matches[3])
	seconds, _ := strconv.Atoi(matches[4])

	return days*86400 + hours*3600 + minutes*60 + seconds
}
