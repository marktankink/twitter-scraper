package twitterscraper_test

import (
	"io"
	"net/http"
	"os"
	"testing"
)

func TestPhotoUpload(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	// Create temp file
	f, err := os.CreateTemp("", "tmp_*.png")
	if err != nil {
		t.Error(err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	resp, err := http.Get("https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png")
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		t.Error(err)
	}

	media, err := testScraper.UploadMedia(f.Name())
	if err != nil {
		t.Error(err)
	}

	if media.ID == 0 {
		t.Error("Media ID shouldn't be 0")
	}
}

func TestVideoUpload(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	// Create temp file
	f, err := os.CreateTemp("", "tmp_*.mp4")
	if err != nil {
		t.Error(err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	resp, err := http.Get("https://github.com/chthomos/video-media-samples/raw/master/big-buck-bunny-480p-30sec.mp4")
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		t.Error(err)
	}

	media, err := testScraper.UploadMedia(f.Name())
	if err != nil {
		t.Error(err)
	}

	if media.ID == 0 {
		t.Error("Media ID shouldn't be 0")
	}
}

func TestGifUpload(t *testing.T) {
	if skipAuthTest {
		t.Skip("Skipping test due to environment variable")
	}

	// Create temp file
	f, err := os.CreateTemp("", "tmp_*.gif")
	if err != nil {
		t.Error(err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	resp, err := http.Get("https://i.giphy.com/dNKC0e3QFNPZC.gif")
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		t.Error(err)
	}

	media, err := testScraper.UploadMedia(f.Name())
	if err != nil {
		t.Error(err)
	}

	if media.ID == 0 {
		t.Error("Media ID shouldn't be 0")
	}
}
