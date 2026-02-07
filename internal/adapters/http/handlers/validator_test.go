package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	SetupValidator() // Ensure validators are registered
}

// ============================================
// Test Custom Validators
// ============================================

func TestValidateCurrencyCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupValidator()

	type TestRequest struct {
		Currency string `json:"currency" binding:"required,currency_code"`
	}

	t.Run("ValidCurrency", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"currency": req.Currency})
		})

		validCodes := []string{"USD", "EUR", "BTC", "ETH"}
		for _, code := range validCodes {
			body, _ := json.Marshal(TestRequest{Currency: code})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Currency %s should be valid", code)
		}
	})

	t.Run("InvalidCurrency_TooShort", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{})
		})

		body, _ := json.Marshal(TestRequest{Currency: "US"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidCurrency_TooLong", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{})
		})

		body, _ := json.Marshal(TestRequest{Currency: "USDT1"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidCurrency_Lowercase", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{})
		})

		body, _ := json.Marshal(TestRequest{Currency: "usd"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestValidateMoneyAmount(t *testing.T) {
	type TestRequest struct {
		Amount string `json:"amount" binding:"required,money_amount"`
	}

	t.Run("ValidAmounts", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{})
		})

		validAmounts := []string{"100", "100.50", "0.01", "1000000.12345678"}
		for _, amount := range validAmounts {
			body, _ := json.Marshal(TestRequest{Amount: amount})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Amount %s should be valid", amount)
		}
	})

	t.Run("InvalidAmounts", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req TestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{})
		})

		invalidAmounts := []string{"-100", "abc", "100.123456789", ""}
		for _, amount := range invalidAmounts {
			body, _ := json.Marshal(TestRequest{Amount: amount})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, "Amount %s should be invalid", amount)
		}
	})
}

// ============================================
// Test Pagination
// ============================================

func TestDefaultPaginationParams(t *testing.T) {
	params := DefaultPaginationParams()

	assert.Equal(t, 1, params.Page)
	assert.Equal(t, 20, params.PerPage)
}

func TestPaginationParams_Offset(t *testing.T) {
	tests := []struct {
		page     int
		perPage  int
		expected int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 10, 20},
		{5, 50, 200},
	}

	for _, tt := range tests {
		params := PaginationParams{Page: tt.page, PerPage: tt.perPage}
		assert.Equal(t, tt.expected, params.Offset())
	}
}

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DefaultValues", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		params := ParsePagination(c)

		assert.Equal(t, 1, params.Page)
		assert.Equal(t, 20, params.PerPage)
	})

	t.Run("CustomValues", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/test?page=3&per_page=50", nil)

		params := ParsePagination(c)

		assert.Equal(t, 3, params.Page)
		assert.Equal(t, 50, params.PerPage)
	})

	t.Run("InvalidPage_UsesDefault", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/test?page=abc", nil)

		params := ParsePagination(c)

		assert.Equal(t, 1, params.Page)
	})

	t.Run("ExceedsMaxPerPage", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/test?per_page=200", nil)

		params := ParsePagination(c)

		assert.Equal(t, 20, params.PerPage) // Should use default when exceeds max
	})
}

func TestBuildMeta(t *testing.T) {
	t.Run("FullPages", func(t *testing.T) {
		params := PaginationParams{Page: 1, PerPage: 20}
		meta := BuildMeta(params, 100)

		assert.Equal(t, 1, meta.Page)
		assert.Equal(t, 20, meta.PerPage)
		assert.Equal(t, 100, meta.Total)
		assert.Equal(t, 5, meta.TotalPages)
	})

	t.Run("PartialLastPage", func(t *testing.T) {
		params := PaginationParams{Page: 1, PerPage: 20}
		meta := BuildMeta(params, 95)

		assert.Equal(t, 5, meta.TotalPages) // 95 / 20 = 4.75, should round up to 5
	})

	t.Run("ExactPages", func(t *testing.T) {
		params := PaginationParams{Page: 1, PerPage: 25}
		meta := BuildMeta(params, 100)

		assert.Equal(t, 4, meta.TotalPages)
	})
}

// ============================================
// Test Bind Functions
// ============================================

func TestBindJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type TestRequest struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}

	t.Run("Success", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		body := []byte(`{"name":"John","email":"john@example.com"}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("X-Request-ID", "test-123")

		var req TestRequest
		result := BindJSON(c, &req)

		assert.True(t, result)
		assert.Equal(t, "John", req.Name)
	})

	t.Run("ValidationError", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := []byte(`{"name":"John"}`) // Missing email
		c.Request = httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("X-Request-ID", "test-123")

		var req TestRequest
		result := BindJSON(c, &req)

		assert.False(t, result)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestBindURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type URIParams struct {
		ID string `uri:"id" binding:"required,uuid"`
	}

	t.Run("Success", func(t *testing.T) {
		router := gin.New()
		router.GET("/users/:id", func(c *gin.Context) {
			c.Set("X-Request-ID", "test-123")
			var params URIParams
			if BindURI(c, &params) {
				c.JSON(200, gin.H{"id": params.ID})
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/users/550e8400-e29b-41d4-a716-446655440000", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		router := gin.New()
		router.GET("/users/:id", func(c *gin.Context) {
			c.Set("X-Request-ID", "test-123")
			var params URIParams
			if !BindURI(c, &params) {
				return
			}
			c.JSON(200, gin.H{})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestValidateKYCStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupValidator()

	type TestRequest struct {
		Status string `json:"status" binding:"required,kyc_status"`
	}

	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		var req TestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": req.Status})
	})

	t.Run("ValidStatuses", func(t *testing.T) {
		validStatuses := []string{"UNVERIFIED", "PENDING", "VERIFIED", "REJECTED"}
		for _, status := range validStatuses {
			body, _ := json.Marshal(TestRequest{Status: status})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Status %s should be valid", status)
		}
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		body, _ := json.Marshal(TestRequest{Status: "INVALID"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestValidateWalletStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupValidator()

	type TestRequest struct {
		Status string `json:"status" binding:"required,wallet_status"`
	}

	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		var req TestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": req.Status})
	})

	t.Run("ValidStatuses", func(t *testing.T) {
		validStatuses := []string{"ACTIVE", "SUSPENDED", "LOCKED", "CLOSED"}
		for _, status := range validStatuses {
			body, _ := json.Marshal(TestRequest{Status: status})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Status %s should be valid", status)
		}
	})

	t.Run("InvalidStatus", func(t *testing.T) {
		body, _ := json.Marshal(TestRequest{Status: "INVALID"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestValidateTransactionType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupValidator()

	type TestRequest struct {
		Type string `json:"type" binding:"required,transaction_type"`
	}

	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		var req TestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"type": req.Type})
	})

	t.Run("ValidTypes", func(t *testing.T) {
		validTypes := []string{"DEPOSIT", "WITHDRAW", "PAYOUT", "TRANSFER", "FEE", "REFUND", "ADJUSTMENT"}
		for _, txType := range validTypes {
			body, _ := json.Marshal(TestRequest{Type: txType})
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Type %s should be valid", txType)
		}
	})

	t.Run("InvalidType", func(t *testing.T) {
		body, _ := json.Marshal(TestRequest{Type: "INVALID"})
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestBindQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type QueryParams struct {
		Status string `form:"status" binding:"required"`
		Page   int    `form:"page" binding:"min=1"`
	}

	t.Run("Success", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("X-Request-ID", "test-123")
			var params QueryParams
			if BindQuery(c, &params) {
				c.JSON(200, gin.H{"status": params.Status, "page": params.Page})
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test?status=active&page=2", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MissingRequired", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("X-Request-ID", "test-123")
			var params QueryParams
			if !BindQuery(c, &params) {
				return
			}
			c.JSON(200, gin.H{})
		})

		req := httptest.NewRequest(http.MethodGet, "/test?page=1", nil) // missing status
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"10", 10},
		{"123", 123},
		{"999", 999},
		{"abc", 0},
		{"12a", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValidationMessage(t *testing.T) {
	// This tests the getValidationMessage function indirectly through validation errors
	gin.SetMode(gin.TestMode)
	SetupValidator()

	type TestRequest struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `json:"name" binding:"required,min=2,max=50"`
		Currency string `json:"currency" binding:"currency_code"`
		Amount   string `json:"amount" binding:"money_amount"`
	}

	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		var req TestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			HandleValidationErrors(c, err)
			return
		}
		c.JSON(200, gin.H{})
	})

	t.Run("EmailValidation", func(t *testing.T) {
		body := []byte(`{"email":"invalid","name":"Test","currency":"USD","amount":"100"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "email")
	})

	t.Run("MinValidation", func(t *testing.T) {
		body := []byte(`{"email":"test@test.com","name":"A","currency":"USD","amount":"100"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "short")
	})
}
