package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	twitterscraper "github.com/imperatrona/twitter-scraper"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get auth tokens from environment variables
	authToken := os.Getenv("TWITTER_AUTH_TOKEN_1")
	csrfToken := os.Getenv("TWITTER_CSRF_TOKEN_1")

	if authToken == "" || csrfToken == "" {
		log.Fatal("TWITTER_AUTH_TOKEN_1 or TWITTER_CSRF_TOKEN_1 environment variables are not set")
	}

	log.Printf("Using tokens (first 4 chars) - Auth: %s... CSRF: %s...",
		authToken[:4], csrfToken[:4])

	// Initialize scraper
	scraper := twitterscraper.New()

	// Create and set cookies directly
	expires := time.Now().Add(365 * 24 * time.Hour)
	cookies := []*http.Cookie{
		{
			Name:     "auth_token",
			Value:    authToken,
			Path:     "/",
			Domain:   "twitter.com",
			Expires:  expires,
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:     "ct0",
			Value:    csrfToken,
			Path:     "/",
			Domain:   "twitter.com",
			Expires:  expires,
			Secure:   true,
			HttpOnly: false,
		},
	}

	// First clear any existing cookies
	scraper.ClearCookies()
	scraper.SetCookies(cookies)

	// Verify cookies were set
	currentCookies := scraper.GetCookies()
	for _, cookie := range currentCookies {
		log.Printf("Cookie set: %s = %s...", cookie.Name, cookie.Value[:4])
	}

	// Try to get a profile first as a test
	testProfile, err := scraper.GetProfile("altcoindealer")
	if err != nil {
		log.Printf("Test profile fetch failed: %v", err)
	} else {
		log.Printf("Successfully fetched test profile - name: %s", testProfile.Name)
	}

	// Now check login status
	if !scraper.IsLoggedIn() {
		log.Fatal("Failed to authenticate with provided tokens")
	}

	// If we get here, authentication worked
	log.Printf("Successfully authenticated!")

	// Username to scrape (default to "x" if no argument provided)
	username := "altcoindealer"
	if len(os.Args) > 1 {
		username = os.Args[1]
	}

	// Get profile
	profile, err := scraper.GetProfile(username)
	if err != nil {
		log.Fatal("Error getting profile:", err)
	}

	// Print profile information
	fmt.Printf("\nProfile Information for @%s:\n", profile.Username)
	fmt.Printf("Name: %s\n", profile.Name)
	fmt.Printf("Bio: %s\n", profile.Biography)
	fmt.Printf("Location: %s\n", profile.Location)
	fmt.Printf("Website: %s\n", profile.Website)
	fmt.Printf("Joined: %v\n", profile.Joined)
	fmt.Printf("Followers: %d\n", profile.FollowersCount)
	fmt.Printf("Following: %d\n", profile.FollowingCount)
	fmt.Printf("Tweets: %d\n", profile.TweetsCount)
	fmt.Printf("Verified: %v\n", profile.IsVerified)
	fmt.Printf("Private: %v\n", profile.IsPrivate)
}
