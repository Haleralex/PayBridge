// Package handlers - Telegram Mini App authentication handler.
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Haleralex/wallethub/internal/adapters/http/common"
	"github.com/Haleralex/wallethub/internal/adapters/http/middleware"
	"github.com/Haleralex/wallethub/internal/application/ports"
	"github.com/Haleralex/wallethub/internal/domain/entities"
	domainErrors "github.com/Haleralex/wallethub/internal/domain/errors"
	"github.com/gin-gonic/gin"
)

// TelegramAuthHandler handles Telegram Mini App authentication.
type TelegramAuthHandler struct {
	userRepo    ports.UserRepository
	walletRepo  ports.WalletRepository
	botToken    string
	jwtSecret   string
	jwtIssuer   string
	tokenExpiry time.Duration
}

// TelegramAuthConfig holds configuration for TelegramAuthHandler.
type TelegramAuthConfig struct {
	UserRepo    ports.UserRepository
	WalletRepo  ports.WalletRepository
	BotToken    string
	JWTSecret   string
	JWTIssuer   string
	TokenExpiry time.Duration
}

// NewTelegramAuthHandler creates a new TelegramAuthHandler.
func NewTelegramAuthHandler(cfg TelegramAuthConfig) *TelegramAuthHandler {
	expiry := cfg.TokenExpiry
	if expiry == 0 {
		expiry = 15 * time.Minute
	}
	return &TelegramAuthHandler{
		userRepo:    cfg.UserRepo,
		walletRepo:  cfg.WalletRepo,
		botToken:    cfg.BotToken,
		jwtSecret:   cfg.JWTSecret,
		jwtIssuer:   cfg.JWTIssuer,
		tokenExpiry: expiry,
	}
}

// TelegramAuthRequest - request body for Telegram authentication.
type TelegramAuthRequest struct {
	InitData string `json:"init_data" binding:"required"`
}

// TelegramAuthResponse - response for Telegram authentication.
type TelegramAuthResponse struct {
	Token  string          `json:"token"`
	UserID string          `json:"user_id"`
	User   TelegramUserDTO `json:"user"`
	IsNew  bool            `json:"is_new"`
}

// TelegramUserDTO - user data in auth response.
type TelegramUserDTO struct {
	ID        string `json:"id"`
	FullName  string `json:"full_name"`
	KYCStatus string `json:"kyc_status"`
}

// TelegramWebAppUser represents user data from Telegram initData.
type TelegramWebAppUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Authenticate handles POST /api/v1/auth/telegram
// Validates Telegram initData, finds or creates user, returns auth token.
func (h *TelegramAuthHandler) Authenticate(c *gin.Context) {
	var req TelegramAuthRequest
	if !BindJSON(c, &req) {
		return
	}

	// 1. Parse and validate initData
	tgUser, err := h.validateInitData(req.InitData)
	if err != nil {
		common.Error(c, http.StatusUnauthorized, &common.APIError{
			Code:    "INVALID_TELEGRAM_DATA",
			Message: fmt.Sprintf("Invalid Telegram data: %v", err),
		})
		return
	}

	// 2. Find existing user by Telegram ID
	isNew := false
	user, err := h.userRepo.FindByTelegramID(c.Request.Context(), tgUser.ID)
	if err != nil {
		if !domainErrors.IsNotFound(err) {
			common.Error(c, http.StatusInternalServerError, &common.APIError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to lookup user",
			})
			return
		}

		// 3. Create new user for this Telegram account
		fullName := tgUser.FirstName
		if tgUser.LastName != "" {
			fullName += " " + tgUser.LastName
		}

		user, err = entities.NewTelegramUser(tgUser.ID, fullName)
		if err != nil {
			common.Error(c, http.StatusInternalServerError, &common.APIError{
				Code:    "USER_CREATION_FAILED",
				Message: fmt.Sprintf("Failed to create user: %v", err),
			})
			return
		}

		if err := h.userRepo.Save(c.Request.Context(), user); err != nil {
			common.Error(c, http.StatusInternalServerError, &common.APIError{
				Code:    "USER_SAVE_FAILED",
				Message: fmt.Sprintf("Failed to save user: %v", err),
			})
			return
		}

		isNew = true
	}

	// 4. Generate real JWT token
	token, err := middleware.GenerateJWT(
		h.jwtSecret,
		h.jwtIssuer,
		user.ID().String(),
		"", // email not available from Telegram
		"user",
		h.tokenExpiry,
	)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, &common.APIError{
			Code:    "TOKEN_GENERATION_FAILED",
			Message: "Failed to generate authentication token",
		})
		return
	}

	response := TelegramAuthResponse{
		Token:  token,
		UserID: user.ID().String(),
		User: TelegramUserDTO{
			ID:        user.ID().String(),
			FullName:  user.FullName(),
			KYCStatus: string(user.KYCStatus()),
		},
		IsNew: isNew,
	}

	common.Success(c, http.StatusOK, response)
}

// validateInitData validates Telegram WebApp initData using HMAC-SHA256.
// See: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func (h *TelegramAuthHandler) validateInitData(initData string) (*TelegramWebAppUser, error) {
	// Parse the query string
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse init_data: %w", err)
	}

	// Extract hash
	hash := values.Get("hash")
	if hash == "" {
		return nil, fmt.Errorf("hash not found in init_data")
	}

	// Extract user
	userData := values.Get("user")
	if userData == "" {
		return nil, fmt.Errorf("user not found in init_data")
	}

	// Check auth_date is not too old (5 minutes max)
	authDate := values.Get("auth_date")
	if authDate == "" {
		return nil, fmt.Errorf("auth_date not found in init_data")
	}

	// Validate HMAC signature — botToken is REQUIRED
	if h.botToken == "" {
		return nil, fmt.Errorf("telegram bot token is not configured")
	}

	// Build data-check-string: sorted key=value pairs excluding "hash"
	var pairs []string
	for key := range values {
		if key == "hash" {
			continue
		}
		pairs = append(pairs, key+"="+values.Get(key))
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// secret_key = HMAC-SHA256(bot_token, "WebAppData")
	secretKeyHMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyHMAC.Write([]byte(h.botToken))
	secretKey := secretKeyHMAC.Sum(nil)

	// hash = HMAC-SHA256(data_check_string, secret_key)
	dataHMAC := hmac.New(sha256.New, secretKey)
	dataHMAC.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(dataHMAC.Sum(nil))

	if !hmac.Equal([]byte(calculatedHash), []byte(hash)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Validate auth_date freshness (max 5 minutes)
	authTimestamp, err := strconv.ParseInt(authDate, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid auth_date format")
	}
	authTime := time.Unix(authTimestamp, 0)
	if time.Since(authTime) > 5*time.Minute {
		return nil, fmt.Errorf("auth_date is too old (possible replay attack)")
	}

	// Parse user JSON
	var tgUser TelegramWebAppUser
	if err := json.Unmarshal([]byte(userData), &tgUser); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	if tgUser.ID == 0 {
		return nil, fmt.Errorf("invalid telegram user ID")
	}

	return &tgUser, nil
}
