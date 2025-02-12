package twitterscraper

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Scraper object
type Scraper struct {
	bearerToken    string
	client         *http.Client
	delay          int64
	guestToken     string
	guestCreatedAt time.Time
	includeReplies bool
	isLogged       bool
	isOpenAccount  bool
	oAuthToken     string
	oAuthSecret    string
	proxy          string
	userAgent      string
	searchMode     SearchMode
	wg             sync.WaitGroup
	cursorTracker  map[string]string // maps username -> cursor
	cursorMutex    sync.RWMutex      // protects cursorTracker
}

// SearchMode type
type SearchMode int

const (
	// SearchTop - default mode
	SearchTop SearchMode = iota
	// SearchLatest - live mode
	SearchLatest
	// SearchPhotos - image mode
	SearchPhotos
	// SearchVideos - video mode
	SearchVideos
	// SearchUsers - user mode
	SearchUsers
)

// default http client timeout
const DefaultClientTimeout = 10 * time.Second
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"

// New creates a Scraper object
func New() *Scraper {
	jar, _ := cookiejar.New(nil)
	return &Scraper{
		bearerToken: bearerToken,
		userAgent:   DefaultUserAgent,
		client: &http.Client{
			Jar:     jar,
			Timeout: DefaultClientTimeout,
		},
	}
}

func (s *Scraper) SaveCursor(username, cursor string) {
	s.cursorMutex.Lock()
	defer s.cursorMutex.Unlock()
	if s.cursorTracker == nil {
		s.cursorTracker = make(map[string]string)
	}
	s.cursorTracker[username] = cursor
}

func (s *Scraper) LoadCursor(username string) string {
	s.cursorMutex.RLock()
	defer s.cursorMutex.RUnlock()
	if s.cursorTracker == nil {
		return ""
	}
	return s.cursorTracker[username]
}

func (s *Scraper) SaveCursorsToFile(filename string) error {
	s.cursorMutex.RLock()
	defer s.cursorMutex.RUnlock()

	data, err := json.MarshalIndent(s.cursorTracker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func (s *Scraper) LoadCursorsFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	s.cursorMutex.Lock()
	defer s.cursorMutex.Unlock()

	return json.Unmarshal(data, &s.cursorTracker)
}

func (s *Scraper) setBearerToken(token string) {
	s.bearerToken = token
	s.guestToken = ""
}

// IsGuestToken check if guest token not empty
func (s *Scraper) IsGuestToken() bool {
	return s.guestToken != ""
}

// SetSearchMode switcher
func (s *Scraper) SetSearchMode(mode SearchMode) *Scraper {
	s.searchMode = mode
	return s
}

// WithDelay add delay between API requests (in seconds)
func (s *Scraper) WithDelay(seconds int64) *Scraper {
	s.delay = seconds
	return s
}

// WithReplies enable/disable load timeline with tweet replies
func (s *Scraper) WithReplies(b bool) *Scraper {
	s.includeReplies = b
	return s
}

// client timeout
func (s *Scraper) WithClientTimeout(timeout time.Duration) *Scraper {
	s.client.Timeout = timeout
	return s
}

// SetProxy
// set http proxy in the format `http://HOST:PORT`
// set socket proxy in the format `socks5://HOST:PORT`
func (s *Scraper) SetProxy(proxyAddr string) error {
	if proxyAddr == "" {
		s.client.Transport = &http.Transport{
			TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
			DialContext: (&net.Dialer{
				Timeout: s.client.Timeout,
			}).DialContext,
		}
		s.proxy = ""
		return nil
	}
	if strings.HasPrefix(proxyAddr, "http") {
		urlproxy, err := url.Parse(proxyAddr)
		if err != nil {
			return err
		}
		s.client.Transport = &http.Transport{
			Proxy:        http.ProxyURL(urlproxy),
			TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
			DialContext: (&net.Dialer{
				Timeout: s.client.Timeout,
			}).DialContext,
		}
		s.proxy = proxyAddr
		return nil
	}
	if strings.HasPrefix(proxyAddr, "socks5") {
		baseDialer := &net.Dialer{
			Timeout:   s.client.Timeout,
			KeepAlive: s.client.Timeout,
		}
		proxyURL, err := url.Parse(proxyAddr)
		if err != nil {
			panic(err)
		}

		// username password
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()

		// ip and port
		host := proxyURL.Hostname()
		port := proxyURL.Port()

		var auth *proxy.Auth

		if username != "" || password != "" {
			auth = &proxy.Auth{
				User:     username,
				Password: password,
			}
		}

		dialSocksProxy, err := proxy.SOCKS5("tcp", host+":"+port, auth, baseDialer)
		if err != nil {
			return errors.New("error creating socks5 proxy :" + err.Error())
		}
		if contextDialer, ok := dialSocksProxy.(proxy.ContextDialer); ok {
			dialContext := contextDialer.DialContext
			s.client.Transport = &http.Transport{
				DialContext: dialContext,
			}
		} else {
			return errors.New("failed type assertion to DialContext")
		}
		s.proxy = proxyAddr
		return nil
	}
	return errors.New("only support http(s) or socks5 protocol")
}

// VerifyProxyConnection checks if the scraper is actually using the configured proxy
func (s *Scraper) VerifyProxyConnection(expectedIP string) error {
	// Use the scraper's own client
	req, err := http.NewRequest("GET", "https://api.ipify.org?format=text", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Use the scraper's own client to make the request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get IP: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	actualIP, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read IP: %v", err)
	}

	// Extract just the IP part from the expected IP (remove port if present)
	expectedIPOnly := strings.Split(expectedIP, ":")[0]
	actualIPStr := strings.TrimSpace(string(actualIP))

	// Compare IPs
	if actualIPStr != expectedIPOnly {
		return fmt.Errorf("scraper not using proxy IP! Expected: %s, Got: %s", expectedIPOnly, actualIPStr)
	}

	return nil
}

func (s *Scraper) SetUserAgent(userAgent string) {
	s.userAgent = userAgent
}

func (s *Scraper) GetUserAgent() string {
	return s.userAgent
}
