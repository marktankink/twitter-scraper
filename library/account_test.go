package twitterscraper_test

import (
	"testing"
)

func TestGetAccountSettings(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	settings, err := testScraper.GetAccountSettings()
	if err != nil {
		t.Error(err)
	}

	if settings.ScreenName == "" {
		t.Error("ScreenName is empty")
	}
}

func TestGetAccountList(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	accounts, err := testScraper.GetAccountList()
	if err != nil {
		t.Error(err)
	}

	if len(accounts) < 1 {
		t.Errorf("Returned %d accounts", len(accounts))
	}
}
