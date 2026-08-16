package waha

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/knadh/listmonk/internal/utils"
	"github.com/knadh/listmonk/models"
)

// Options represents WAHA messenger options.
type Options struct {
	Name              string        `json:"name"`
	Host              string        `json:"host"`
	RootURL           string        `json:"root_url"`
	APIKey            string        `json:"api_key"`
	Session           string        `json:"session"`
	PhoneAttribute    string        `json:"phone_attribute"`
	TypingDelayMs     int           `json:"typing_delay_ms"`
	TargetWPM         int           `json:"target_wpm"`
	WPMStd            float64       `json:"wpm_std"`
	KeyboardLayout    string        `json:"keyboard_layout"`
	TypingMode        string        `json:"typing_mode"`
	MaxTypingDelaySec int           `json:"max_typing_delay_sec"`
	MaxConns          int           `json:"max_conns"`
	Retries           int           `json:"retries"`
	Timeout           time.Duration `json:"timeout"`
}

// Waha represents a WAHA messenger backend.
type Waha struct {
	o Options
	c *http.Client
}

type chatRequest struct {
	Session string `json:"session"`
	ChatID  string `json:"chatId"`
}

type sendTextRequest struct {
	Session string `json:"session"`
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
}

type sendImageRequest struct {
	Session string      `json:"session"`
	ChatID  string      `json:"chatId"`
	Caption string      `json:"caption,omitempty"`
	File    filePayload `json:"file"`
}

type filePayload struct {
	MIMEType string `json:"mimetype"`
	Filename string `json:"filename"`
	Data     string `json:"data"`
}

// New returns a new instance of the WAHA messenger with zero-config defaults.
func New(o Options) (*Waha, error) {
	if o.RootURL == "" && o.Host != "" {
		o.RootURL = o.Host
	}
	if o.Host == "" && o.RootURL != "" {
		o.Host = o.RootURL
	}
	if o.Session == "" {
		o.Session = "default"
	}
	if o.PhoneAttribute == "" {
		o.PhoneAttribute = "phone"
	}
	if o.TypingDelayMs <= 0 {
		o.TypingDelayMs = 50
	}
	if o.TargetWPM <= 0 {
		o.TargetWPM = 60
	}
	if o.WPMStd <= 0 {
		o.WPMStd = 10.0
	}
	if o.KeyboardLayout == "" {
		o.KeyboardLayout = "qwerty"
	}
	if o.TypingMode == "" {
		o.TypingMode = "human"
	}
	if o.MaxTypingDelaySec <= 0 {
		o.MaxTypingDelaySec = 30
	}
	if o.MaxConns <= 0 {
		o.MaxConns = 10
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Second
	}

	return &Waha{
		o: o,
		c: &http.Client{
			Timeout: o.Timeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost:   o.MaxConns,
				MaxConnsPerHost:       o.MaxConns,
				ResponseHeaderTimeout: o.Timeout,
				IdleConnTimeout:       o.Timeout,
			},
		},
	}, nil
}

// SetHTTPClient sets a custom HTTP client for testing / VCR transport.
func (w *Waha) SetHTTPClient(client *http.Client) {
	if client != nil {
		w.c = client
	}
}

// Name returns the messenger's name.
func (w *Waha) Name() string {
	return w.o.Name
}

// Push pushes a message to the WAHA server using human typing simulation and keep-alives.
func (w *Waha) Push(m models.Message) error {
	phone := m.ToPhone
	if phone == "" {
		phone = m.Subscriber.Phone.String
	}
	if phone == "" {
		phone = extractPhone(m.Subscriber.Attribs, w.o.PhoneAttribute)
	}
	if phone == "" {
		return fmt.Errorf("subscriber %s missing phone attribute '%s'", m.Subscriber.UUID, w.o.PhoneAttribute)
	}

	chatID, err := formatChatID(phone)
	if err != nil {
		return fmt.Errorf("subscriber %s has invalid phone: %w", m.Subscriber.UUID, err)
	}
	session := w.o.Session
	if m.MessengerSession != "" {
		session = m.MessengerSession
	}

	// Step 1: Simulate human typing indicator lifecycle
	switch w.o.TypingMode {
	case "off":
		// No typing simulation
	case "simple":
		_ = w.startTyping(chatID, session)
		time.Sleep(time.Duration(w.o.TypingDelayMs) * time.Millisecond)
		_ = w.stopTyping(chatID, session)
	default: // "human" (Default)
		typingDelay := calculateHumanTypingDelay(m.Body, w.o)
		_ = w.startTyping(chatID, session)

		// Start periodic keep-alive ticker every 8 seconds for long delays
		stopTicker := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(8 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = w.startTyping(chatID, session)
				case <-stopTicker:
					return
				}
			}
		}()

		time.Sleep(typingDelay)
		close(stopTicker)
		wg.Wait()

		_ = w.stopTyping(chatID, session)
	}

	// Step 2: Convert content and dispatch payload
	formattedText := convertToWhatsAppMarkdown(string(m.Body))
	if len(m.Attachments) > 0 {
		return w.sendImage(chatID, session, formattedText, m.Attachments[0])
	}

	return w.sendText(chatID, session, formattedText)
}

// Flush flushes the message queue.
func (w *Waha) Flush() error {
	return nil
}

// Close closes idle HTTP connections.
func (w *Waha) Close() error {
	if w.c != nil {
		w.c.CloseIdleConnections()
	}
	return nil
}

func (w *Waha) startTyping(chatID, session string) error {
	url := fmt.Sprintf("%s/api/startTyping", strings.TrimRight(w.o.RootURL, "/"))
	req := chatRequest{Session: session, ChatID: chatID}
	return w.post(url, req)
}

func (w *Waha) stopTyping(chatID, session string) error {
	url := fmt.Sprintf("%s/api/stopTyping", strings.TrimRight(w.o.RootURL, "/"))
	req := chatRequest{Session: session, ChatID: chatID}
	return w.post(url, req)
}

func (w *Waha) sendText(chatID, session, text string) error {
	url := fmt.Sprintf("%s/api/sendText", strings.TrimRight(w.o.RootURL, "/"))
	req := sendTextRequest{Session: session, ChatID: chatID, Text: text}
	return w.post(url, req)
}

func (w *Waha) sendImage(chatID, session, caption string, att models.Attachment) error {
	url := fmt.Sprintf("%s/api/sendImage", strings.TrimRight(w.o.RootURL, "/"))
	req := sendImageRequest{
		Session: session,
		ChatID:  chatID,
		Caption: caption,
		File: filePayload{
			MIMEType: att.Header.Get("Content-Type"),
			Filename: att.Name,
			Data:     fmt.Sprintf("data:%s;base64,%s", att.Header.Get("Content-Type"), base64.StdEncoding.EncodeToString(att.Content)),
		},
	}
	return w.post(url, req)
}

func (w *Waha) post(url string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	retries := w.o.Retries
	if retries <= 0 {
		retries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * 500 * time.Millisecond)
		}

		httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
		if err != nil {
			return err
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if w.o.APIKey != "" {
			httpReq.Header.Set("X-Api-Key", w.o.APIKey)
		}

		resp, err := w.c.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("WAHA error (%d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 || resp.StatusCode == 429 {
				continue
			}
			return lastErr
		}

		return nil
	}

	return lastErr
}

func (w *Waha) put(url string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	retries := w.o.Retries
	if retries <= 0 {
		retries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * 500 * time.Millisecond)
		}

		httpReq, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(b))
		if err != nil {
			return err
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if w.o.APIKey != "" {
			httpReq.Header.Set("X-Api-Key", w.o.APIKey)
		}

		resp, err := w.c.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("WAHA error (%d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 || resp.StatusCode == 429 {
				continue
			}
			return lastErr
		}

		return nil
	}

	return lastErr
}

type webhookItem struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type sessionConfigPayload struct {
	Config struct {
		Webhooks []webhookItem `json:"webhooks"`
	} `json:"config"`
}

// SyncWebhook ensures the WAHA session is configured with Listmonk's webhook URL for message events.
func (w *Waha) SyncWebhook(publicURL string) error {
	if w.o.RootURL == "" {
		return nil
	}
	webhookURL := strings.TrimRight(publicURL, "/") + "/api/webhooks/waha"
	session := w.o.Session
	if session == "" {
		session = "default"
	}

	payload := sessionConfigPayload{}
	payload.Config.Webhooks = []webhookItem{
		{
			URL:    webhookURL,
			Events: []string{"message.ack", "message"},
		},
	}

	urlPut := fmt.Sprintf("%s/api/sessions/%s", strings.TrimRight(w.o.RootURL, "/"), session)
	err := w.put(urlPut, payload)
	if err == nil {
		return nil
	}

	urlStart := fmt.Sprintf("%s/api/sessions/start", strings.TrimRight(w.o.RootURL, "/"))
	startPayload := struct {
		Name   string               `json:"name"`
		Config sessionConfigPayload `json:"config"`
	}{
		Name:   session,
		Config: payload,
	}

	err = w.post(urlStart, startPayload)
	if err != nil && (strings.Contains(err.Error(), "already started") || strings.Contains(err.Error(), "422")) {
		return nil
	}
	return err
}

// --- HUMAN TYPING MARKOV SIMULATION MODEL ---

const (
	probError             = 0.04
	probSwapError         = 0.015
	probNoticeError       = 0.85
	driftCorrectionProb   = 0.8
	complexWordErrorMult  = 1.5
	commonWordErrorMult   = 0.5
	composedAccentErrMult = 2.0

	speedBoostCommonWord = 0.6
	speedPenaltyComplex  = 1.3
	speedBoostCloseKeys  = 0.5
	speedBoostBigram     = 0.4
	closeKeyThreshold    = 2.0
	farKeyThreshold      = 4.0
	farKeyPenalty        = 1.2
	minSpeedMultiplier   = 0.15

	timeKeystrokeStd          = 0.03
	timeBackspaceMean         = 0.12
	timeBackspaceStd          = 0.02
	timeReactionMean          = 0.35
	timeReactionStd           = 0.10
	timeDirectAccentPenalty   = 0.15
	timeComposedAccentPenalty = 0.40
	timeUppercasePenalty      = 0.20
	timeSpacePauseMean        = 0.25
	timeSpacePauseStd         = 0.05

	minKeystrokeTime = 0.02
	minReactionTime  = 0.10
	minBackspaceTime = 0.03

	fatigueFactor = 1.0005
	fatigueCap    = 1.5
)

type keyboardLayoutMap struct {
	grid            [][]rune
	posMap          map[rune][2]int
	directAccents   map[rune]bool
	composedAccents map[rune]bool
}

func newKeyboardLayout(layoutName string) *keyboardLayoutMap {
	var grid [][]rune
	if strings.ToLower(layoutName) == "azerty" {
		grid = [][]rune{
			[]rune("&é\"'(-è_çà)="),
			[]rune("azertyuiop^$"),
			[]rune("qsdfghjklmù*"),
			[]rune("wxcvbn,;:!"),
		}
	} else {
		// Default QWERTY
		grid = [][]rune{
			[]rune("`1234567890-="),
			[]rune("qwertyuiop[]\\"),
			[]rune("asdfghjkl;'"),
			[]rune("zxcvbnm,./"),
		}
	}

	posMap := make(map[rune][2]int)
	for r, row := range grid {
		for c, char := range row {
			posMap[char] = [2]int{r, c}
		}
	}

	directAccents := make(map[rune]bool)
	composedAccents := make(map[rune]bool)
	if strings.ToLower(layoutName) == "azerty" {
		for _, r := range "éèàùç" {
			directAccents[r] = true
		}
		for _, r := range "âêîôûäëïöü" {
			composedAccents[r] = true
		}
	} else {
		for _, r := range "âêîôûäëïöüéèàùç" {
			composedAccents[r] = true
		}
	}

	return &keyboardLayoutMap{
		grid:            grid,
		posMap:          posMap,
		directAccents:   directAccents,
		composedAccents: composedAccents,
	}
}

func (k *keyboardLayoutMap) normalizeChar(r rune) rune {
	return unicode.ToLower(r)
}

func (k *keyboardLayoutMap) hasKey(r rune) bool {
	_, ok := k.posMap[k.normalizeChar(r)]
	return ok
}

func (k *keyboardLayoutMap) getDistance(r1, r2 rune) float64 {
	p1, ok1 := k.posMap[k.normalizeChar(r1)]
	p2, ok2 := k.posMap[k.normalizeChar(r2)]
	if !ok1 || !ok2 {
		return farKeyThreshold
	}
	dr := float64(p1[0] - p2[0])
	dc := float64(p1[1] - p2[1])
	return math.Sqrt(dr*dr + dc*dc)
}

func (k *keyboardLayoutMap) getRandomNeighbor(r rune) rune {
	norm := k.normalizeChar(r)
	pos, ok := k.posMap[norm]
	if !ok {
		return 'e'
	}

	deltas := [][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	var neighbors []rune
	for _, d := range deltas {
		nr, nc := pos[0]+d[0], pos[1]+d[1]
		if nr >= 0 && nr < len(k.grid) && nc >= 0 && nc < len(k.grid[nr]) {
			neighbors = append(neighbors, k.grid[nr][nc])
		}
	}

	if len(neighbors) == 0 {
		return 'e'
	}

	res := neighbors[rand.Intn(len(neighbors))]
	if unicode.IsUpper(r) {
		return unicode.ToUpper(res)
	}
	return res
}

// Common English words for speed boost
var commonWords = map[string]bool{
	"the": true, "be": true, "to": true, "of": true, "and": true, "a": true, "in": true, "that": true, "have": true, "it": true,
	"for": true, "not": true, "on": true, "with": true, "he": true, "as": true, "you": true, "do": true, "at": true, "this": true,
	"but": true, "his": true, "by": true, "from": true, "they": true, "we": true, "say": true, "her": true, "she": true, "or": true,
	"an": true, "will": true, "my": true, "one": true, "all": true, "would": true, "there": true, "their": true, "what": true,
	"so": true, "up": true, "out": true, "if": true, "about": true, "who": true, "get": true, "which": true, "go": true, "me": true,
	"when": true, "make": true, "can": true, "like": true, "time": true, "no": true, "just": true, "him": true, "know": true,
	"take": true, "people": true, "into": true, "year": true, "your": true, "good": true, "some": true, "could": true,
	"them": true, "see": true, "other": true, "than": true, "then": true, "now": true, "look": true, "only": true, "come": true,
	"its": true, "over": true, "think": true, "also": true, "back": true, "after": true, "use": true, "two": true, "how": true,
	"our": true, "work": true, "first": true, "well": true, "way": true, "even": true, "new": true, "want": true, "because": true,
}

var commonBigrams = map[string]bool{
	"th": true, "he": true, "in": true, "er": true, "an": true, "re": true, "on": true, "at": true, "en": true, "nd": true, "ti": true, "es": true,
	"or": true, "te": true, "of": true, "ed": true, "is": true, "it": true, "al": true, "ar": true, "st": true, "to": true, "nt": true, "ng": true,
	"se": true, "ha": true, "as": true, "ou": true, "io": true, "le": true, "ve": true, "co": true, "me": true, "de": true, "hi": true, "ri": true,
	"ro": true, "ic": true, "ne": true, "ea": true, "ra": true, "ce": true,
}

func getWordDifficulty(word string) string {
	cleaned := strings.Trim(strings.ToLower(word), ".,!?;:'\"-()[]{}")
	if commonWords[cleaned] {
		return "common"
	}
	if len(cleaned) > 8 || strings.ContainsAny(cleaned, "zxqj") {
		return "complex"
	}
	return "normal"
}

func isCommonBigram(r1, r2 rune) bool {
	bg := strings.ToLower(string([]rune{r1, r2}))
	return commonBigrams[bg]
}

func normFloat64(mean, std float64) float64 {
	return mean + rand.NormFloat64()*std
}

type markovState struct {
	currentText       []rune
	targetText        []rune
	totalTimeSec      float64
	fatigueMultiplier float64
	mentalCursorPos   int
	lastCharTyped     rune
	lastActionWasBack bool
}

type markovTyper struct {
	targetText       string
	keyboard         *keyboardLayoutMap
	sessionWPM       float64
	baseKeystrokeSec float64
	state            markovState
}

func newMarkovTyper(targetText string, targetWPM float64, wpmStd float64, layout string) *markovTyper {
	sessionWPM := normFloat64(targetWPM, wpmStd)
	if sessionWPM < 10 {
		sessionWPM = 10
	}
	baseKS := 60.0 / (sessionWPM * 5.0)

	return &markovTyper{
		targetText:       targetText,
		keyboard:         newKeyboardLayout(layout),
		sessionWPM:       sessionWPM,
		baseKeystrokeSec: baseKS,
		state: markovState{
			currentText:       nil,
			targetText:        []rune(targetText),
			totalTimeSec:      0,
			fatigueMultiplier: 1.0,
			mentalCursorPos:   0,
			lastCharTyped:     0,
			lastActionWasBack: false,
		},
	}
}

func (m *markovTyper) getCurrentWordContext() string {
	idx := m.state.mentalCursorPos
	runes := m.state.targetText
	if idx >= len(runes) {
		return ""
	}
	start := idx
	for start > 0 && runes[start-1] != ' ' {
		start--
	}
	end := idx
	for end < len(runes) && runes[end] != ' ' {
		end++
	}
	return string(runes[start:end])
}

func (m *markovTyper) calculateKeystrokeTime(charToType rune) float64 {
	ksTime := m.baseKeystrokeSec * m.state.fatigueMultiplier

	wordCtx := m.getCurrentWordContext()
	if wordCtx != "" {
		diff := getWordDifficulty(wordCtx)
		if diff == "common" {
			ksTime *= speedBoostCommonWord
		} else if diff == "complex" {
			ksTime *= speedPenaltyComplex
		}
	}

	if m.state.lastCharTyped != 0 {
		if isCommonBigram(m.state.lastCharTyped, charToType) {
			ksTime *= speedBoostBigram
		} else {
			dist := m.keyboard.getDistance(m.state.lastCharTyped, charToType)
			if dist > 0 && dist < closeKeyThreshold {
				ksTime *= speedBoostCloseKeys
			} else if dist > farKeyThreshold {
				ksTime *= farKeyPenalty
			}
		}
	}

	if charToType == ' ' {
		ksTime += math.Max(0, normFloat64(timeSpacePauseMean, timeSpacePauseStd))
	} else if m.keyboard.composedAccents[charToType] {
		ksTime += timeComposedAccentPenalty
	} else if m.keyboard.directAccents[charToType] {
		ksTime += timeDirectAccentPenalty
	} else if unicode.IsUpper(charToType) {
		ksTime += timeUppercasePenalty
	}

	ksTime = math.Max(minSpeedMultiplier*m.baseKeystrokeSec, ksTime)
	dt := normFloat64(ksTime, timeKeystrokeStd)
	return math.Max(minKeystrokeTime, dt)
}

func (m *markovTyper) step() bool {
	if string(m.state.currentText) == string(m.state.targetText) {
		return false
	}

	// 1. Error Detection & Correction Phase
	firstErrPos := len(m.state.targetText)
	minLen := len(m.state.currentText)
	if len(m.state.targetText) < minLen {
		minLen = len(m.state.targetText)
	}

	for i := 0; i < minLen; i++ {
		if m.state.currentText[i] != m.state.targetText[i] {
			firstErrPos = i
			break
		}
	}
	if len(m.state.currentText) > len(m.state.targetText) && firstErrPos == len(m.state.targetText) {
		firstErrPos = len(m.state.targetText)
	}

	if firstErrPos < len(m.state.currentText) {
		shouldCorrect := false

		if m.state.lastActionWasBack {
			shouldCorrect = true
		} else if m.state.mentalCursorPos >= len(m.state.targetText) {
			shouldCorrect = true
		} else if len(m.state.currentText) > 0 {
			lastChar := m.state.currentText[len(m.state.currentText)-1]
			dist := len(m.state.currentText) - firstErrPos

			if strings.ContainsRune(" \n\t.,;!?:()[]{}<>'\"", lastChar) {
				shouldCorrect = true
			} else if dist >= 2 {
				if rand.Float64() < driftCorrectionProb {
					shouldCorrect = true
				}
			} else if dist == 1 {
				if rand.Float64() < probNoticeError {
					shouldCorrect = true
				}
			}
		}

		if shouldCorrect {
			if !m.state.lastActionWasBack {
				dt := math.Max(minReactionTime, normFloat64(timeReactionMean, timeReactionStd))
				m.state.totalTimeSec += dt
			}

			dt := math.Max(minBackspaceTime, normFloat64(timeBackspaceMean, timeBackspaceStd))
			m.state.totalTimeSec += dt
			m.state.currentText = m.state.currentText[:len(m.state.currentText)-1]
			m.state.mentalCursorPos = len(m.state.currentText)
			m.state.lastActionWasBack = true
			return true
		}
	}

	m.state.lastActionWasBack = false

	// 2. Typing Phase
	if m.state.mentalCursorPos > len(m.state.currentText) {
		m.state.mentalCursorPos = len(m.state.currentText)
	}
	if m.state.mentalCursorPos >= len(m.state.targetText) {
		return false
	}

	charIntended := m.state.targetText[m.state.mentalCursorPos]

	if !m.keyboard.hasKey(charIntended) && charIntended != ' ' {
		m.state.fatigueMultiplier = math.Min(fatigueCap, m.state.fatigueMultiplier*fatigueFactor)
		dt := m.baseKeystrokeSec * m.state.fatigueMultiplier
		dt = math.Max(minKeystrokeTime, normFloat64(dt, timeKeystrokeStd))
		m.state.totalTimeSec += dt
		m.state.currentText = append(m.state.currentText, charIntended)
		m.state.lastCharTyped = charIntended
		m.state.mentalCursorPos++
		return true
	}

	m.state.fatigueMultiplier = math.Min(fatigueCap, m.state.fatigueMultiplier*fatigueFactor)

	// Swap anticipation error
	if len(m.state.targetText) > m.state.mentalCursorPos+1 {
		charAfter := m.state.targetText[m.state.mentalCursorPos+1]
		if charAfter != ' ' && charAfter != charIntended {
			if rand.Float64() < probSwapError {
				dt1 := m.calculateKeystrokeTime(charAfter)
				m.state.totalTimeSec += dt1
				m.state.currentText = append(m.state.currentText, charAfter)

				dt2 := m.calculateKeystrokeTime(charIntended)
				m.state.totalTimeSec += dt2
				m.state.currentText = append(m.state.currentText, charIntended)

				m.state.lastCharTyped = charIntended
				m.state.mentalCursorPos += 2
				return true
			}
		}
	}

	// Normal typing (success vs neighbor mistyping)
	currProbErr := probError
	wordDiff := getWordDifficulty(m.getCurrentWordContext())
	if wordDiff == "complex" {
		currProbErr *= complexWordErrorMult
	} else if wordDiff == "common" {
		currProbErr *= commonWordErrorMult
	}
	if m.keyboard.composedAccents[charIntended] {
		currProbErr *= composedAccentErrMult
	}

	if rand.Float64() < currProbErr {
		wrongChar := m.keyboard.getRandomNeighbor(charIntended)
		dt := m.calculateKeystrokeTime(wrongChar)
		m.state.totalTimeSec += dt
		m.state.currentText = append(m.state.currentText, wrongChar)
		m.state.lastCharTyped = wrongChar
		m.state.mentalCursorPos++
	} else {
		dt := m.calculateKeystrokeTime(charIntended)
		m.state.totalTimeSec += dt
		m.state.currentText = append(m.state.currentText, charIntended)
		m.state.lastCharTyped = charIntended
		m.state.mentalCursorPos++
	}

	return true
}

func (m *markovTyper) run() float64 {
	steps := 0
	maxSteps := len(m.state.targetText) * 10
	for m.step() {
		steps++
		if steps > maxSteps {
			break
		}
	}
	return m.state.totalTimeSec
}

// calculateHumanTypingDelay runs the MarkovTyper simulation and returns the calculated duration bounded within limits.
func calculateHumanTypingDelay(body []byte, o Options) time.Duration {
	text := string(body)
	if len(strings.TrimSpace(text)) == 0 {
		return 500 * time.Millisecond
	}

	typer := newMarkovTyper(text, float64(o.TargetWPM), o.WPMStd, o.KeyboardLayout)
	totalSec := typer.run()

	maxSec := float64(o.MaxTypingDelaySec)
	if maxSec <= 0 {
		maxSec = 30.0
	}

	if totalSec < 1.0 {
		totalSec = 1.0
	} else if totalSec > maxSec {
		totalSec = maxSec
	}

	return time.Duration(totalSec * float64(time.Second))
}

func formatChatID(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", fmt.Errorf("empty phone number")
	}

	target := phone
	if strings.HasSuffix(target, "@c.us") {
		target = strings.TrimSuffix(target, "@c.us")
	}

	sanitized, err := utils.SanitizePhone(target)
	if err != nil {
		return "", fmt.Errorf("invalid WhatsApp recipient phone '%s'", phone)
	}

	digits := strings.TrimPrefix(sanitized, "+")
	return digits + "@c.us", nil
}

func extractPhone(attribs models.JSON, key string) string {
	if val, ok := attribs[key].(string); ok {
		return val
	}
	return ""
}

func convertToWhatsAppMarkdown(input string) string {
	s := input
	s = regexp.MustCompile(`(?i)<b>|<strong>`).ReplaceAllString(s, "*")
	s = regexp.MustCompile(`(?i)</b>|</strong>`).ReplaceAllString(s, "*")
	s = regexp.MustCompile(`(?i)<i>|<em>`).ReplaceAllString(s, "_")
	s = regexp.MustCompile(`(?i)</i>|</em>`).ReplaceAllString(s, "_")
	s = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
