package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/joho/godotenv"
)

// Tracking API calls ------------------------------------------
type APICallCounter struct {
	TimelineCalls uint64            // GetTweets timeline requests
	ThreadCalls   uint64            // Individual thread detail requests
	TotalCalls    uint64            // Total API requests
	AccountCalls  map[string]uint64 // Calls per account (using AuthToken as key)
}

func NewAPICallCounter(accounts []AccountInfo) *APICallCounter {
	accountCalls := make(map[string]uint64)
	for _, acc := range accounts {
		accountCalls[acc.AuthToken] = 0
	}
	return &APICallCounter{
		AccountCalls: accountCalls,
	}
}

func (c *APICallCounter) IncrementAccount(authToken string) {
	// Get current value
	current := c.AccountCalls[authToken]
	// Update with new value
	c.AccountCalls[authToken] = current + 1
}

func (c *APICallCounter) PrintStats(w io.Writer) {
	fmt.Fprintf(w, "\nAPI Call Statistics:\n")
	fmt.Fprintf(w, "Timeline API Calls: %d\n", atomic.LoadUint64(&c.TimelineCalls))
	fmt.Fprintf(w, "Thread Detail API Calls: %d\n", atomic.LoadUint64(&c.ThreadCalls))
	fmt.Fprintf(w, "Total API Calls: %d\n", atomic.LoadUint64(&c.TimelineCalls)+atomic.LoadUint64(&c.ThreadCalls))
	fmt.Fprintf(w, "\nPer Account API Calls:\n")
	for token, calls := range c.AccountCalls {
		// Only show last 4 chars of token for privacy
		fmt.Fprintf(w, "Account (token ending ...%s): %d calls\n", token[len(token)-4:], calls)
	}
}

// Main Tweet struct
type TweetOutput struct {
	ID            string        `json:"id"`
	UserName      string        `json:"user_name"`
	Text          string        `json:"text"`
	Metrics       TweetMetrics  `json:"metrics"`
	URLs          []string      `json:"urls,omitempty"`
	Photos        []string      `json:"photos"`
	VideoURLs     []string      `json:"video_urls,omitempty"`
	PermanentURL  string        `json:"tweet_url"`
	TimeParsed    time.Time     `json:"time_parsed"`
	Timestamp     int64         `json:"time_stamp"`
	IsQuoted      bool          `json:"is_quoted"`
	QuotedTweet   *QuotedTweet  `json:"quoted_tweet,omitempty"`
	IsThreadStart bool          `json:"is_thread_start"`
	ThreadTweets  []ThreadTweet `json:"thread_tweets,omitempty"`
}

type TweetMetrics struct {
	Likes   int `json:"likes"`
	Replies int `json:"replies"`
	Views   int `json:"views"`
}

type ThreadTweet struct {
	ID     string   `json:"id"`
	Text   string   `json:"text"`
	Likes  int      `json:"likes"`
	Photos []string `json:"photos,omitempty"`
	Videos []string `json:"videos,omitempty"`
}

type QuotedTweet struct {
	ID           string   `json:"id"`
	UserName     string   `json:"user_name"`
	Text         string   `json:"text"`
	Photos       []string `json:"photos,omitempty"`
	Videos       []string `json:"videos,omitempty"`
	PermanentURL string   `json:"tweet_url"`
}

type AccountInfo struct {
	AuthToken string
	CSRFToken string
}

// Thread tracking
type ThreadTracker struct {
	processedThreads map[string]bool
	processedTweets  map[string]bool
	targetUserID     string
}

func NewThreadTracker(userID string) *ThreadTracker {
	return &ThreadTracker{
		processedThreads: make(map[string]bool),
		processedTweets:  make(map[string]bool),
		targetUserID:     userID,
	}
}

func (t *ThreadTracker) isProcessedTweet(id string) bool {
	return t.processedTweets[id]
}

func (t *ThreadTracker) isProcessedThread(id string) bool {
	return t.processedThreads[id]
}

func (t *ThreadTracker) markTweetProcessed(id string) {
	t.processedTweets[id] = true
}

func (t *ThreadTracker) markThreadProcessed(thread *twitterscraper.Tweet) {
	t.processedThreads[thread.ID] = true
	t.markTweetProcessed(thread.ID)

	// Mark all thread tweets as processed
	for _, threadTweet := range thread.Thread {
		t.markTweetProcessed(threadTweet.ID)
	}
}

// extractMediaURLs helper function to extract media URLs from a tweet
func extractMediaURLs(tweet *twitterscraper.Tweet) ([]string, []string) {
	photoURLs := make([]string, 0)
	for _, photo := range tweet.Photos {
		photoURLs = append(photoURLs, photo.URL)
	}

	videoURLs := make([]string, 0)
	for _, video := range tweet.Videos {
		if video.URL != "" {
			videoURLs = append(videoURLs, video.URL)
		}
	}

	return photoURLs, videoURLs
}

func validateAccounts(accounts []AccountInfo) error {
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts provided")
	}

	for i, acc := range accounts {
		if acc.AuthToken == "" || acc.CSRFToken == "" {
			return fmt.Errorf("missing tokens for account %d", i+1)
		}
	}
	return nil
}

// Set up proxy URL
func getProxyURL() string {
	username := os.Getenv("PROXY_USERNAME")
	password := os.Getenv("PROXY_PASSWORD")
	proxyIP := os.Getenv("PROXY_IP")

	if username == "" || password == "" || proxyIP == "" {
		log.Fatal("Missing proxy configuration in .env file")
	}

	// Construct proxy URL with authentication
	return fmt.Sprintf("http://%s:%s@%s", username, password, proxyIP)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Set scraper settings
	username := "Simpelyfe"
	tweetLimit := 5

	scraper := twitterscraper.New() // Initialize new scraper
	scraper.WithDelay(1)            // Add a small delay between requests to avoid rate limiting

	// Load any previously saved cursors
	if err := scraper.LoadCursorsFromFile("cursors.json"); err != nil {
		log.Printf("Warning: Could not load cursors: %v", err)
	}

	// ALL PROXY OPERATIONS ------------------------------------
	// Set up proxy
	proxyURL := getProxyURL()
	if err := scraper.SetProxy(proxyURL); err != nil {
		log.Fatalf("Failed to set proxy: %v", err)
	}
	// Verify proxy using the new library method
	proxyIP := os.Getenv("PROXY_IP")
	if err := scraper.VerifyProxyConnection(proxyIP); err != nil {
		log.Fatalf("Proxy verification failed: %v", err)
	}
	log.Println("Successfully verified scraper is using proxy")

	// ALL ACCOUNT OPERATIONS -----------------------------------
	// Load and validate accounts
	accounts := []AccountInfo{
		{AuthToken: os.Getenv("TWITTER_AUTH_TOKEN_1"), CSRFToken: os.Getenv("TWITTER_CSRF_TOKEN_1")},
		// {AuthToken: os.Getenv("TWITTER_AUTH_TOKEN_2"), CSRFToken: os.Getenv("TWITTER_CSRF_TOKEN_2")},
		// {AuthToken: os.Getenv("TWITTER_AUTH_TOKEN_3"), CSRFToken: os.Getenv("TWITTER_CSRF_TOKEN_3")},
		// {AuthToken: os.Getenv("TWITTER_AUTH_TOKEN_4"), CSRFToken: os.Getenv("TWITTER_CSRF_TOKEN_4")},
		// {AuthToken: os.Getenv("TWITTER_AUTH_TOKEN_5"), CSRFToken: os.Getenv("TWITTER_CSRF_TOKEN_5")},
	}
	if err := validateAccounts(accounts); err != nil {
		log.Fatalf("Account validation failed: %v", err)
	}

	currentAccount := 0

	// // Test first account connection
	// account := accounts[0]
	// scraper.SetAuthToken(twitterscraper.AuthToken{
	// 	Token:     account.AuthToken,
	// 	CSRFToken: account.CSRFToken,
	// })
	// if !scraper.IsLoggedIn() {
	// 	log.Fatal("Failed to authenticate with Twitter")
	// }

	// -------------------------------------------------------
	account := accounts[0]
	log.Printf("Attempting authentication with tokens (first 4 chars) - Auth: %s... CSRF: %s...",
		account.AuthToken[:4], account.CSRFToken[:4])

	// First clear any existing cookies
	scraper.ClearCookies()

	// Set the auth tokens
	scraper.SetAuthToken(twitterscraper.AuthToken{
		Token:     account.AuthToken,
		CSRFToken: account.CSRFToken,
	})

	// Verify cookie setup
	cookies := scraper.GetCookies()
	foundAuth := false
	foundCSRF := false
	for _, cookie := range cookies {
		if cookie.Name == "auth_token" {
			foundAuth = true
			log.Printf("Found auth_token cookie: %s...", cookie.Value[:4])
		}
		if cookie.Name == "ct0" {
			foundCSRF = true
			log.Printf("Found ct0 (CSRF) cookie: %s...", cookie.Value[:4])
			// Verify the CSRF token matches what we set
			if cookie.Value != account.CSRFToken {
				log.Printf("Warning: CSRF token in cookie (%s...) doesn't match provided token (%s...)",
					cookie.Value[:4], account.CSRFToken[:4])
			}
		}
	}

	if !foundAuth || !foundCSRF {
		log.Fatal("Cookies were not properly set")
	}

	// Try getting a profile as a test, but first make sure we have valid tokens
	if foundAuth && foundCSRF {
		// Try a real user profile - like Twitter's own profile
		profile, err := scraper.GetProfile("Altcoindealer")
		if err != nil {
			log.Printf("Test profile request failed: %v", err)
		} else {
			log.Printf("Successfully fetched Twitter profile - name: %s", profile.Name)
		}
	}

	if !scraper.IsLoggedIn() {
		log.Fatal("Failed to authenticate with Twitter")
	}
	// ------------------------------------------------------------

	// // Get user profile to get UserID
	profile, err := scraper.GetProfile(username)
	if err != nil {
		log.Fatal("Failed to get user profile:", err)
	}

	// Initialize trackers
	counter := NewAPICallCounter(accounts)
	tracker := NewThreadTracker(profile.UserID)
	var outputTweets []TweetOutput
	startTime := time.Now()

	// GetTweets makes paginated requests, each fetching up to 20 tweets So we'll increment TimelineCalls for each page
	atomic.AddUint64(&counter.TimelineCalls, 1)
	lastCount := 0

	// Start scraping tweets
	// Main scraping loop
	for tweet := range scraper.GetTweets(context.Background(), username, tweetLimit) {
		// Track pagination calls
		currentCount := len(outputTweets)
		if currentCount/20 > lastCount/20 {
			atomic.AddUint64(&counter.TimelineCalls, 1)
			lastCount = currentCount
			// Add a small delay between detailed tweet requests to avoid rate limiting
			time.Sleep(time.Second * 10)
		}

		// Rotate account before request
		account := accounts[currentAccount]
		scraper.SetAuthToken(twitterscraper.AuthToken{
			Token:     account.AuthToken,
			CSRFToken: account.CSRFToken,
		})
		counter.IncrementAccount(account.AuthToken)
		currentAccount = (currentAccount + 1) % len(accounts)

		if tweet.Error != nil {
			log.Printf("Error fetching tweet: %v", tweet.Error)
			continue
		}

		// Skip processed tweets
		if tracker.isProcessedTweet(tweet.ID) {
			continue
		}

		if tweet.ID != tweet.ConversationID {
			// Handle thread tweets
			if tracker.isProcessedThread(tweet.ConversationID) {
				continue
			}

			fullThread, err := scraper.GetTweet(tweet.ConversationID)
			atomic.AddUint64(&counter.ThreadCalls, 1)
			if err != nil {
				log.Printf("Error fetching thread %s: %v", tweet.ConversationID, err)
				continue
			}

			photoURLs, videoURLs := extractMediaURLs(fullThread)

			if fullThread.IsSelfThread && len(fullThread.Thread) > 0 {
				threadOutput := TweetOutput{
					ID:       fullThread.ID,
					UserName: fullThread.Username,
					Text:     fullThread.Text,
					Metrics: TweetMetrics{
						Likes:   fullThread.Likes,
						Replies: fullThread.Replies,
						Views:   fullThread.Views,
					},
					URLs:          fullThread.URLs,
					Photos:        photoURLs,
					VideoURLs:     videoURLs,
					PermanentURL:  fullThread.PermanentURL,
					TimeParsed:    fullThread.TimeParsed,
					Timestamp:     fullThread.Timestamp,
					IsThreadStart: fullThread.IsSelfThread,
					IsQuoted:      fullThread.IsQuoted,
					QuotedTweet: func() *QuotedTweet {
						if fullThread.IsQuoted && fullThread.QuotedStatus != nil {
							quotedPhotoURLs, quotedVideoURLs := extractMediaURLs(fullThread.QuotedStatus)
							return &QuotedTweet{
								ID:           fullThread.QuotedStatus.ID,
								UserName:     fullThread.QuotedStatus.Username,
								Text:         fullThread.QuotedStatus.Text,
								Photos:       quotedPhotoURLs,
								Videos:       quotedVideoURLs,
								PermanentURL: fullThread.QuotedStatus.PermanentURL,
							}
						}
						return nil
					}(),
				}

				for _, threadTweet := range fullThread.Thread {
					threadPhotoURLs, threadVideoURLs := extractMediaURLs(threadTweet)
					threadOutput.ThreadTweets = append(threadOutput.ThreadTweets, ThreadTweet{
						ID:     threadTweet.ID,
						Text:   threadTweet.Text,
						Likes:  threadTweet.Likes,
						Photos: threadPhotoURLs,
						Videos: threadVideoURLs,
					})
				}

				// Remove the last element from the output since this was the start of the thread but as a seperate tweet.
				outputTweets = outputTweets[:len(outputTweets)-1]

				outputTweets = append(outputTweets, threadOutput)
				tracker.markThreadProcessed(fullThread)

				// Add a small delay after making api request
				time.Sleep(time.Second * 5)
			}
		} else {
			// Handle standalone tweets
			photoURLs, videoURLs := extractMediaURLs(&tweet.Tweet)
			outputTweets = append(outputTweets, TweetOutput{
				ID:       tweet.ID,
				UserName: tweet.Username,
				Text:     tweet.Text,
				Metrics: TweetMetrics{
					Likes:   tweet.Likes,
					Replies: tweet.Replies,
					Views:   tweet.Views,
				},
				URLs:          tweet.URLs,
				Photos:        photoURLs,
				VideoURLs:     videoURLs,
				PermanentURL:  tweet.PermanentURL,
				TimeParsed:    tweet.TimeParsed,
				Timestamp:     tweet.Timestamp,
				IsThreadStart: tweet.IsSelfThread,
				IsQuoted:      tweet.IsQuoted,
				QuotedTweet: func() *QuotedTweet {
					if tweet.IsQuoted && tweet.QuotedStatus != nil {
						quotedPhotoURLs, quotedVideoURLs := extractMediaURLs(tweet.QuotedStatus)
						return &QuotedTweet{
							ID:           tweet.QuotedStatus.ID,
							UserName:     tweet.QuotedStatus.Username,
							Text:         tweet.QuotedStatus.Text,
							Photos:       quotedPhotoURLs,
							Videos:       quotedVideoURLs,
							PermanentURL: tweet.QuotedStatus.PermanentURL,
						}
					}
					return nil
				}(),
			})
			tracker.markTweetProcessed(tweet.ID)
		}
		log.Printf("Scraped tweet ID %s using account ending in %s", tweet.ID, account.AuthToken[len(account.AuthToken)-4:])
	}

	// Save cursors for next run
	if err := scraper.SaveCursorsToFile("cursors.json"); err != nil {
		log.Printf("Error saving cursors: %v", err)
	}

	// Create JSON file --------------------------------------------------------------------
	// Write JSON output
	timestamp := time.Now().Format("2006-01-02_15-04")
	outputJsonFile := fmt.Sprintf("json/tweets_%s_%s.json", username, timestamp)
	file, err := os.Create(outputJsonFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(outputTweets); err != nil {
		log.Fatalf("Error encoding tweets to JSON: %v", err)
	}

	// Write all stats to a separate file -------------------------------------------------
	duration := time.Since(startTime)
	outputStatsFile := fmt.Sprintf("scrape_logs/stats_%s_%s.txt", username, timestamp)
	stats, err := os.Create(outputStatsFile)
	if err != nil {
		log.Fatalf("Error creating stats file: %v", err)
	}
	defer stats.Close()
	// Write statistics to file
	fmt.Fprintf(stats, "Scraping Summary:\n")
	fmt.Fprintf(stats, "Total Tweets Processed: %d\n", len(outputTweets))
	fmt.Fprintf(stats, "Duration: %v\n", duration.Round(time.Second))
	fmt.Fprintf(stats, "Average Rate: %.2f tweets/minute\n",
		float64(len(outputTweets))/(duration.Minutes()))
	counter.PrintStats(stats)

	fmt.Printf("Successfully scraped tweets for %s and saved to %s\n", username, outputJsonFile)
}
