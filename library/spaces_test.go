package twitterscraper_test

import (
	"errors"
	"testing"
)

func TestGetSpace(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	spaceId := "1OdJrXPVLEnKX"

	space, err := testScraper.GetSpace(spaceId)
	if err != nil {
		t.Fatal(err)
	}

	if space.ID != spaceId {
		t.Fatal(errors.New("returned space id is not requested"))
	}
}
