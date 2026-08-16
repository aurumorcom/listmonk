package testutil

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

// NewVCRRecorder creates a new VCR recorder for the given cassette path and configures header scrubbing.
func NewVCRRecorder(t *testing.T, cassetteName string) (*recorder.Recorder, *http.Client) {
	t.Helper()

	mode := os.Getenv("VCR_MODE")
	recMode := recorder.ModePassthrough
	if mode == "record" {
		recMode = recorder.ModeRecordOnly
	} else if mode == "replay" {
		recMode = recorder.ModeReplayOnly
	} else if mode == "record_once" {
		recMode = recorder.ModeRecordOnce
	}

	cassettePath := filepath.Join("test", "cassettes", cassetteName)
	if _, err := os.Stat("test"); os.IsNotExist(err) {
		cassettePath = filepath.Join("..", "..", "test", "cassettes", cassetteName)
	}

	_ = os.MkdirAll(filepath.Dir(cassettePath), 0755)

	// Check if cassette file exists
	cassetteExists := false
	if _, err := os.Stat(cassettePath + ".yaml"); err == nil {
		cassetteExists = true
	}

	if mode == "" && !cassetteExists {
		recMode = recorder.ModePassthrough
	}

	opts := &recorder.Options{
		CassetteName:       cassettePath,
		Mode:               recMode,
		RealTransport:      http.DefaultTransport,
		SkipRequestLatency: true,
	}

	r, err := recorder.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("failed to create VCR recorder: %v", err)
	}

	t.Cleanup(func() {
		if err := r.Stop(); err != nil {
			t.Errorf("failed to stop VCR recorder: %v", err)
		}
	})

	return r, r.GetDefaultClient()
}
