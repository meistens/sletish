package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sletish/internal/models"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	jikanAPIURL        = "https://api.jikan.moe/v4"
	defaultTimeout     = 30 * time.Second
	rateLimitDelay     = 1 * time.Second
	maxRetries         = 3
	retryDelay         = 2 * time.Second
	userAgent          = "AnimeTrackerBot/1.0"
	maxSearchResults   = 10
	searchCachePrefix  = "anime:search:"
	detailsCachePrefix = "anime:details:"
	searchCacheTTL     = 4 * time.Hour
	detailsCacheTTL    = 24 * time.Hour
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	logger      *logrus.Logger
	lastRequest time.Time
	rateLimiter chan struct{}
	redis       *redis.Client
}

type ClientConfig struct {
	BaseURL    string
	Timeout    time.Duration
	RateLimit  time.Duration
	MaxRetries int
	RetryDelay time.Duration
	UserAgent  string
	Logger     *logrus.Logger
	Redis      *redis.Client
}

func NewClient() *Client {
	return NewClientWithConfig(&ClientConfig{
		BaseURL:    jikanAPIURL,
		Timeout:    defaultTimeout,
		RateLimit:  rateLimitDelay,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
		UserAgent:  userAgent,
		Logger:     logrus.New(),
	})
}

func NewClientWithConfig(config *ClientConfig) *Client {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	client := &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		logger:      config.Logger,
		rateLimiter: make(chan struct{}, 1),
		redis:       config.Redis,
	}
	client.rateLimiter <- struct{}{}
	return client
}

func (c *Client) SearchAnime(query string) (*models.JikanSearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	c.logger.WithField("query", query).Info("Searching anime...")

	// check cache first
	cacheKey := searchCachePrefix + query
	if c.redis != nil {
		cached, err := c.redis.Get(context.Background(), cacheKey).Result()
		if err == nil {
			c.logger.WithField("query", query).Info("Retrieved search results from cache")

			var cachedResponse models.JikanSearchResponse
			if err := json.Unmarshal([]byte(cached), &cachedResponse); err == nil {
				return &cachedResponse, nil
			} else {
				c.logger.WithError(err).Warn("Failed to unmarshal cached search result")
			}
		} else if err != redis.Nil {
			c.logger.WithError(err).Warn("Failed to read from Redis")
		}
	}

	// if no cache, hit API
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(maxSearchResults))
	params.Set("sort", "desc")

	searchURL := fmt.Sprintf("%s/anime?%s", c.baseURL, params.Encode())

	resp, err := c.makeRequest(searchURL)
	if err != nil {
		return nil, err
	}

	var searchResult models.JikanSearchResponse
	if err := json.Unmarshal(resp, &searchResult); err != nil {
		return nil, err
	}

	// cache results
	if c.redis != nil {
		responseJSON, err := json.Marshal(searchResult)
		if err != nil {
			c.logger.WithError(err).Warn("Failed to marshal search result for caching")
		} else {
			if err := c.redis.Set(context.Background(), cacheKey, responseJSON, searchCacheTTL).Err(); err != nil {
				c.logger.WithError(err).Warn("Failed to write search result to cache")
			} else {
				c.logger.WithField("query", query).Debug("Search result cached successfully")
			}
		}
	}

	return &searchResult, nil
}

func FormatAnimeMessage(animes []models.AnimeData) string {
	if len(animes) == 0 {
		return "No anime found for your search query."
	}

	var message strings.Builder
	message.WriteString("<b>🔍 Anime Search Results:</b>\n\n")

	// values above 13 will not work...
	for i, anime := range animes {
		if i >= maxSearchResults {
			break
		}

		// Anime title with number
		message.WriteString(fmt.Sprintf("<b>%d. %s</b>\n", i+1, anime.Title))

		// ID for adding to list
		message.WriteString(fmt.Sprintf("🆔 ID: <code>%d</code>", anime.MalID))

		// Score with star emoji
		if anime.Score > 0 {
			message.WriteString(fmt.Sprintf(" | ⭐ %.1f", anime.Score))
		}

		// Episodes count
		if anime.Episodes > 0 {
			message.WriteString(fmt.Sprintf(" | 📺 %d eps", anime.Episodes))
		}

		// Year
		if anime.Year > 0 {
			message.WriteString(fmt.Sprintf(" | 📅 %d", anime.Year))
		}

		message.WriteString("\n")

		// Type and Status on second line
		var details []string
		if anime.Type != "" {
			details = append(details, fmt.Sprintf("📱 %s", anime.Type))
		}
		if anime.Status != "" {
			details = append(details, fmt.Sprintf("📊 %s", anime.Status))
		}
		if len(details) > 0 {
			message.WriteString(strings.Join(details, " | ") + "\n")
		}

		// Genres
		if len(anime.Genres) > 0 {
			genres := make([]string, 0, len(anime.Genres))
			for _, genre := range anime.Genres {
				genres = append(genres, genre.Name)
			}
			genreText := strings.Join(genres, ", ")
			if len(genreText) > 50 {
				genreText = genreText[:50] + "..."
			}
			message.WriteString(fmt.Sprintf("🏷 %s\n", genreText))
		}

		// Synopsis (shortened)
		if anime.Synopsis != "" {
			synopsis := anime.Synopsis
			if len(synopsis) > 150 {
				synopsis = synopsis[:150] + "..."
			}
			message.WriteString(fmt.Sprintf("📝 %s\n", synopsis))
		}

		// Link to MyAnimeList
		message.WriteString(fmt.Sprintf("🔗 <a href=\"https://myanimelist.net/anime/%d\">View on MyAnimeList</a>\n", anime.MalID))

		// Separator for readability
		if i < len(animes)-1 && i < 9 { // Don't add separator after last item
			message.WriteString("\n━━━━━━━━━━━━━━━━━━━\n\n")
		} else {
			message.WriteString("\n")
		}
	}

	return message.String()
}

func (c *Client) makeRequest(url string) ([]byte, error) {
	var rErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.enforceRateLimit()
		<-c.rateLimiter

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			rErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			rErr = fmt.Errorf("failed to make HTTP request: %w", err)
			c.retryLogger(attempt, url, err)
			c.rateLimiter <- struct{}{}
			c.waitForRetry(attempt)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			rErr = fmt.Errorf("API returned status code %d", resp.StatusCode)
			c.retryLogger(attempt, url, err)
			c.rateLimiter <- struct{}{}
			c.waitForRetry(attempt)
			continue
		}

		body, err := c.readRespBody(resp)
		resp.Body.Close()
		c.rateLimiter <- struct{}{}

		if err != nil {
			rErr = fmt.Errorf("failed to read response body: %w", err)
			c.retryLogger(attempt, url, err)
			c.waitForRetry(attempt)
			continue
		}

		c.logger.WithFields(logrus.Fields{
			"url":           url,
			"attempt":       attempt,
			"status":        resp.StatusCode,
			"response_size": len(body),
		}).Debug("API request successful")

		c.lastRequest = time.Now()
		return body, nil
	}

	return nil, fmt.Errorf("failed %d, attempts: %w", maxRetries, rErr)
}

func (c *Client) enforceRateLimit() {
	now := time.Now()
	if c.lastRequest.Add(rateLimitDelay).After(now) {
		zzzTime := c.lastRequest.Add(rateLimitDelay).Sub(now)
		c.logger.WithField("sleep_time", zzzTime).Debug("Rate limit: sleeping")
		time.Sleep(zzzTime)
	}
}

func (c *Client) retryLogger(attempt int, url string, err error) {
	c.logger.WithFields(logrus.Fields{
		"attempt": attempt + 1,
		"url":     url,
		"error":   err.Error(),
	}).Warn("API request failed, retrying...")
}

func (c *Client) readRespBody(resp *http.Response) ([]byte, error) {
	// limit response size to prevent memory issue
	const maxResponseSize = 5 * 1024 * 1024 // 5MB

	if resp.ContentLength > maxResponseSize {
		return nil, fmt.Errorf("response too large: %d bytes", resp.ContentLength)
	}

	// read with size limit

	var initialCap int64 = 1024 // Default initial capacity
	if resp.ContentLength > 0 && resp.ContentLength <= maxResponseSize {
		initialCap = resp.ContentLength
	}
	body := make([]byte, 0, initialCap)

	buf := make([]byte, 4096)
	totalRead := 0

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			totalRead += n
			if totalRead > maxResponseSize {
				return nil, fmt.Errorf("response too large: exceeded % bytes", maxResponseSize)
			}
			body = append(body, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}

	return body, nil
}

func (c *Client) waitForRetry(attempt int) {
	if attempt < maxRetries-1 {
		delay := time.Duration(attempt+1) * retryDelay
		c.logger.WithField("delay", delay).Debug("waiting before retry")
		time.Sleep(delay)
	}
}

func (c *Client) GetAnimeByID(id int) (*models.AnimeData, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid anime ID: %d", id)
	}

	c.logger.WithField("anime_id", id).Info("Fetching anime by ID...")

	// Check cache first
	cacheKey := detailsCachePrefix + strconv.Itoa(id)
	if c.redis != nil {
		cached, err := c.redis.Get(context.Background(), cacheKey).Result()
		if err == nil {
			c.logger.WithField("anime_id", id).Info("Retrieved anime details from cache")

			var cachedAnime models.AnimeData
			if err := json.Unmarshal([]byte(cached), &cachedAnime); err == nil {
				return &cachedAnime, nil
			} else {
				c.logger.WithError(err).Warn("Failed to unmarshal cached anime details")
			}
		} else if err != redis.Nil {
			c.logger.WithError(err).Warn("Failed to read from Redis")
		}
	}

	// Build the correct URL for single anime endpoint
	reqURL := fmt.Sprintf("%s/anime/%d", c.baseURL, id)

	resp, err := c.makeRequest(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get anime by ID %d: %w", id, err)
	}

	// Single anime endpoint returns different structure than search
	var animeResp struct {
		Data models.AnimeData `json:"data"`
	}

	if err := json.Unmarshal(resp, &animeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anime response for ID %d: %w", id, err)
	}

	// Cache the result
	if c.redis != nil {
		animeJSON, err := json.Marshal(animeResp.Data)
		if err != nil {
			c.logger.WithError(err).Warn("Failed to marshal anime for caching")
		} else {
			if err := c.redis.Set(context.Background(), cacheKey, animeJSON, detailsCacheTTL).Err(); err != nil {
				c.logger.WithError(err).Warn("Failed to write anime details to cache")
			} else {
				c.logger.WithField("anime_id", id).Debug("Anime details cached successfully")
			}
		}
	}

	// Log successful fetch
	c.logger.WithFields(logrus.Fields{
		"anime_id": id,
		"title":    animeResp.Data.Title,
	}).Info("Anime details fetched successfully")

	return &animeResp.Data, nil
}
