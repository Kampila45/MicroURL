package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type TurnstileVerifier struct {
	secretKey string
	client    *http.Client
	isDev     bool
	appEnv    string
}

func NewTurnstileVerifier(secretKey string, isDev bool, appEnv string) *TurnstileVerifier {
	verifier := &TurnstileVerifier{
		secretKey: secretKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		isDev:  isDev,
		appEnv: appEnv,
	}

	if secretKey == "" && isDev {
		verifier.logBypassWarning()
	}

	return verifier
}

func (v *TurnstileVerifier) logBypassWarning() {
	msg := fmt.Sprintf(
		"\n"+
			"╔════════════════════════════════════════════════════════════════╗\n"+
			"║  CRITICAL SECURITY WARNING: CAPTCHA BYPASS MODE ACTIVE        ║\n"+
			"╠════════════════════════════════════════════════════════════════╣\n"+
			"║  Captcha verification is DISABLED in dev mode.                ║\n"+
			"║  This should NEVER happen in production!                      ║\n"+
			"║  Current environment: %-40s ║\n"+
			"║  If this is production, check your APP_ENV configuration.    ║\n"+
			"╚════════════════════════════════════════════════════════════════╝\n",
		v.appEnv,
	)
	fmt.Fprintf(os.Stderr, "[TurnstileVerifier] %s", msg)
}

type verifyResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	if v.secretKey == "" {
		actualEnv := os.Getenv("APP_ENV")
		isExplicitlyDev := v.isDev && (v.appEnv == "development" || actualEnv == "development")

		if isExplicitlyDev {
			return nil
		}

		return fmt.Errorf("captcha verification failed: secret key is missing in non-dev environment (APP_ENV=%s)", v.appEnv)
	}

	if token == "" {
		return fmt.Errorf("captcha token is required")
	}

	data := url.Values{}
	data.Set("secret", v.secretKey)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create verify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call turnstile verify api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {	
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("turnstile verify api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode turnstile response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("captcha verification failed: %v", result.ErrorCodes)
	}

	return nil
}

