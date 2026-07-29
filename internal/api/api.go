package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"youtube-manager/internal/collector"
	"youtube-manager/internal/database"
	"youtube-manager/internal/models"
	"youtube-manager/internal/scheduler"
	"youtube-manager/internal/youtube"
)

type Server struct {
	db        *database.DB
	keyPool   *youtube.KeyPool
	collector *collector.Collector
	sched     *scheduler.Scheduler
	cronSpec  string
}

func NewServer(db *database.DB, keyPool *youtube.KeyPool, collector *collector.Collector, sched *scheduler.Scheduler, cronSpec string) *Server {
	return &Server{
		db:        db,
		keyPool:   keyPool,
		collector: collector,
		sched:     sched,
		cronSpec:  cronSpec,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/stats/summary", s.handleStatsSummary)

	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/auth/me", s.handleAuthMe)

	mux.HandleFunc("/api/users", s.handleUsers)
	mux.HandleFunc("/api/users/", s.handleUserSubroutes)

	mux.HandleFunc("/api/channels", s.handleChannels)
	mux.HandleFunc("/api/channels/", s.handleChannelSubroutes)

	mux.HandleFunc("/api/videos/", s.handleVideoSubroutes)

	mux.HandleFunc("/api/analytics/compare", s.handleAnalyticsCompare)
	mux.HandleFunc("/api/export/channel/", s.handleExportChannelCSV)

	mux.HandleFunc("/api/collector/trigger", s.handleTriggerCollector)
	mux.HandleFunc("/api/keys", s.handleAPIKeys)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "healthy",
		"active_api_keys": s.keyPool.GetActiveCount(),
		"timestamp":       time.Now(),
	})
}

func (s *Server) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetDashboardStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	stats.CronSchedule = s.cronSpec
	if s.sched != nil {
		stats.NextScheduleAt = s.sched.GetNextRunTime()
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channels, err := s.db.GetChannels()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, channels)

	case http.MethodPost:
		var req struct {
			HandleOrID string `json:"handle_or_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.HandleOrID == "" {
			writeError(w, http.StatusBadRequest, "Invalid request body. Field 'handle_or_id' is required.")
			return
		}

		go func(input string) {
			s.collector.SyncChannel(input)
		}(req.HandleOrID)

		// Sync immediately or return quick status
		if err := s.collector.SyncChannel(req.HandleOrID); err != nil {
			writeError(w, http.StatusBadRequest, "Failed adding channel: "+err.Error())
			return
		}

		ch, err := s.db.GetChannelByID(req.HandleOrID)
		if err != nil {
			writeJSON(w, http.StatusAccepted, map[string]string{"message": "Channel sync initiated in background"})
			return
		}

		writeJSON(w, http.StatusCreated, ch)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleChannelSubroutes(w http.ResponseWriter, r *http.Request) {
	// Path: /api/channels/{id} or /api/channels/{id}/videos or /api/channels/{id}/history
	path := strings.TrimPrefix(r.URL.Path, "/api/channels/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Channel ID required")
		return
	}

	channelID := parts[0]

	if len(parts) == 1 {
		// /api/channels/{id}
		if r.Method == http.MethodDelete {
			if err := s.db.DeleteChannel(channelID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "Channel deleted successfully"})
			return
		}

		if r.Method == http.MethodPut {
			if err := s.db.SetPrimaryChannel(channelID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "Channel set as primary My Channel"})
			return
		}

		ch, err := s.db.GetChannelByID(channelID)
		if err != nil {
			writeError(w, http.StatusNotFound, "Channel not found")
			return
		}
		writeJSON(w, http.StatusOK, ch)
		return
	}

	subroute := parts[1]
	switch subroute {
	case "set-primary":
		if err := s.db.SetPrimaryChannel(channelID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Channel set as primary My Channel"})
	case "videos":
		search := r.URL.Query().Get("q")
		sortBy := r.URL.Query().Get("sort")
		videoType := r.URL.Query().Get("type")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		pageResult, err := s.db.GetVideosByChannel(channelID, search, sortBy, videoType, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, pageResult)

	case "history":
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		history, err := s.db.GetChannelHistory(channelID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, history)

	case "hourly-stats":
		stats, err := s.db.GetHourlyPeakStats(channelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)

	default:
		writeError(w, http.StatusNotFound, "Subroute not found")
	}
}

func (s *Server) handleVideoSubroutes(w http.ResponseWriter, r *http.Request) {
	// Path: /api/videos/{id} or /api/videos/{id}/history
	path := strings.TrimPrefix(r.URL.Path, "/api/videos/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Video ID required")
		return
	}

	videoID := parts[0]

	if len(parts) == 1 {
		v, err := s.db.GetVideoByID(videoID)
		if err != nil {
			writeError(w, http.StatusNotFound, "Video not found")
			return
		}
		writeJSON(w, http.StatusOK, v)
		return
	}

	subroute := parts[1]
	if subroute == "history" {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		history, err := s.db.GetVideoHistory(videoID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, history)
		return
	}

	writeError(w, http.StatusNotFound, "Subroute not found")
}

func (s *Server) handleAnalyticsCompare(w http.ResponseWriter, r *http.Request) {
	myChanID := r.URL.Query().Get("my_channel")
	competitorStr := r.URL.Query().Get("competitors")
	var compIDs []string
	if competitorStr != "" {
		for _, c := range strings.Split(competitorStr, ",") {
			if trimmed := strings.TrimSpace(c); trimmed != "" {
				compIDs = append(compIDs, trimmed)
			}
		}
	}

	result, err := s.db.GetCompetitorComparison(myChanID, compIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExportChannelCSV(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/export/channel/")
	channelID := strings.TrimSpace(path)
	if channelID == "" {
		writeError(w, http.StatusBadRequest, "Channel ID required")
		return
	}

	ch, err := s.db.GetChannelByID(channelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Channel not found")
		return
	}

	videos, err := s.db.GetVideosByChannel(channelID, "", "newest", "all", 1, 5000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=channel_%s_stats.csv", ch.ChannelID))

	// Write UTF-8 BOM for Excel compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{"Video ID", "Tiêu Đề", "Ngày Đăng", "Thời Lượng (Giây)", "Lượt Views Mới Nhất", "Lượt Likes Mới Nhất", "Lượt Comments Mới Nhất", "URL Thumbnail"})

	for _, v := range videos.Items {
		writer.Write([]string{
			v.VideoID,
			v.Title,
			v.PublishedAt.Format("2006-01-02 15:04:05"),
			strconv.Itoa(v.DurationSeconds),
			strconv.FormatInt(v.LatestViews, 10),
			strconv.FormatInt(v.LatestLikes, 10),
			strconv.FormatInt(v.LatestComments, 10),
			v.ThumbnailURL,
		})
	}
}

func (s *Server) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}
	return r.URL.Query().Get("token")
}

func (s *Server) getCurrentUser(r *http.Request) (*models.User, error) {
	token := s.extractToken(r)
	if token == "" {
		return nil, fmt.Errorf("authentication required: missing session token")
	}
	user, err := s.db.GetUserBySessionToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session token")
	}
	return user, nil
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil || user.PasswordHash != database.HashPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "Tên đăng nhập hoặc mật khẩu không chính xác")
		return
	}

	session, err := s.db.CreateSession(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed creating user session: "+err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": session.Token,
		"user": models.UserInfo{
			ID:        user.ID,
			Username:  user.Username,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := s.extractToken(r)
	if token != "" {
		s.db.DeleteSession(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.getCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, models.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	currentUser, err := s.getCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if currentUser.Role != "admin" && currentUser.Role != "superadmin" {
		writeError(w, http.StatusForbidden, "Chỉ tài khoản Admin mới có quyền quản lý người dùng")
		return
	}

	switch r.Method {
	case http.MethodGet:
		users, err := s.db.GetUsers(currentUser.Role)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, users)

	case http.MethodPost:
		var req models.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		newUser, err := s.db.CreateUser(req.Username, req.Password, req.Role)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, newUser)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleUserSubroutes(w http.ResponseWriter, r *http.Request) {
	currentUser, err := s.getCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if currentUser.Role != "admin" && currentUser.Role != "superadmin" {
		writeError(w, http.StatusForbidden, "Chỉ tài khoản Admin mới có quyền quản lý người dùng")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/users/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "User ID required")
		return
	}

	userID, _ := strconv.ParseInt(parts[0], 10, 64)

	if len(parts) == 1 {
		if r.Method == http.MethodDelete {
			if userID == currentUser.ID {
				writeError(w, http.StatusBadRequest, "Không thể xóa chính tài khoản đang đăng nhập")
				return
			}
			if err := s.db.DeleteUser(currentUser.Role, userID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
			return
		}
	}

	if len(parts) >= 2 && parts[1] == "password" && r.Method == http.MethodPut {
		var req struct {
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPassword == "" {
			writeError(w, http.StatusBadRequest, "new_password is required")
			return
		}
		if err := s.db.ResetUserPassword(userID, req.NewPassword); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "User password reset successfully"})
		return
	}

	writeError(w, http.StatusNotFound, "Subroute not found")
}

func (s *Server) handleTriggerCollector(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	go func() {
		s.collector.CollectAll()
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Manual data collection triggered in background.",
	})
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, err := s.db.GetActiveAPIKeys()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, keys)

	case http.MethodPost:
		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
			writeError(w, http.StatusBadRequest, "Key value required")
			return
		}

		s.keyPool.AddKey(req.Key)
		s.db.SaveAPIKey(req.Key)

		writeJSON(w, http.StatusCreated, map[string]string{"message": "API key added to rotation pool"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}
