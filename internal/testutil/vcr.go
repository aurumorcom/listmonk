package testutil

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

// NewVCRRecorder creates a new VCR recorder for the given cassette path and configures header scrubbing.
func NewVCRRecorder(t *testing.T, cassetteName string) (*recorder.Recorder, *http.Client) {
	t.Helper()

	LoadDotEnv()

	mode := os.Getenv("VCR_MODE")

	cassettePath := resolveCassettePath(cassetteName)

	_ = os.MkdirAll(filepath.Dir(cassettePath), 0755)

	// Check if cassette file exists
	cassetteExists := false
	if _, err := os.Stat(cassettePath + ".yaml"); err == nil {
		cassetteExists = true
	}

	var recMode recorder.Mode
	switch mode {
	case "record":
		recMode = recorder.ModeRecordOnly
	case "record_once":
		recMode = recorder.ModeRecordOnce
	case "passthrough":
		recMode = recorder.ModePassthrough
	case "replay":
		if cassetteExists {
			recMode = recorder.ModeReplayOnly
		} else {
			recMode = recorder.ModeRecordOnce
		}
	default:
		if cassetteExists {
			recMode = recorder.ModeReplayOnly
		} else {
			recMode = recorder.ModeRecordOnce
		}
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

	// Match requests flexibly by Method and URL path, ignoring header differences for scrubbed auth tokens
	r.SetMatcher(func(req *http.Request, cassReq cassette.Request) bool {
		if req.Method != cassReq.Method {
			return false
		}
		if req.URL.String() == cassReq.URL {
			return true
		}
		if req.URL.Path != "" && strings.HasSuffix(cassReq.URL, req.URL.Path) {
			return true
		}
		return false
	})

	// Scrub sensitive headers before saving interactions
	r.AddHook(func(i *cassette.Interaction) error {
		scrubHeaders(i.Request.Headers)
		scrubHeaders(i.Response.Headers)
		return nil
	}, recorder.AfterCaptureHook)

	t.Cleanup(func() {
		if err := r.Stop(); err != nil {
			t.Errorf("failed to stop VCR recorder: %v", err)
		}
	})

	return r, r.GetDefaultClient()
}

func resolveCassettePath(cassetteName string) string {
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join("test", "cassettes", cassetteName)
	}

	for i := 0; i < 5; i++ {
		testDir := filepath.Join(dir, "test", "cassettes")
		if _, err := os.Stat(testDir); err == nil {
			return filepath.Join(testDir, cassetteName)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Join("test", "cassettes", cassetteName)
}

func scrubHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	sensitive := []string{"x-api-key", "authorization", "api-key", "cookie", "set-cookie"}
	for _, header := range sensitive {
		for key := range headers {
			if strings.EqualFold(key, header) {
				headers[key] = []string{"[REDACTED]"}
			}
		}
	}
}
