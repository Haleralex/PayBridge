package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: func(token string) (*AuthClaims, error) {
				return &AuthClaims{
					UserID: "user-123",
					Email:  "test@example.com",
					Role:   "user",
					Exp:    time.Now().Add(1 * time.Hour),
				}, nil
			},
			SkipPaths: []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MissingAuthHeader", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: MockTokenValidator,
			SkipPaths:      []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("InvalidHeaderFormat", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: MockTokenValidator,
			SkipPaths:      []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("EmptyToken", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: MockTokenValidator,
			SkipPaths:      []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: func(token string) (*AuthClaims, error) {
				return nil, errors.New("invalid token")
			},
			SkipPaths: []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: func(token string) (*AuthClaims, error) {
				return &AuthClaims{
					UserID: "user-123",
					Email:  "test@example.com",
					Role:   "user",
					Exp:    time.Now().Add(-1 * time.Hour), // Expired
				}, nil
			},
			SkipPaths: []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("SkipPaths", func(t *testing.T) {
		config := &AuthConfig{
			TokenValidator: MockTokenValidator,
			SkipPaths:      []string{"/public"},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/public", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "public"})
		})

		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		// No Authorization header
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ClaimsInContext", func(t *testing.T) {
		userID := uuid.New().String()
		email := "test@example.com"
		role := "admin"

		config := &AuthConfig{
			TokenValidator: func(token string) (*AuthClaims, error) {
				return &AuthClaims{
					UserID: userID,
					Email:  email,
					Role:   role,
					Exp:    time.Now().Add(1 * time.Hour),
				}, nil
			},
			SkipPaths: []string{},
		}

		router := gin.New()
		router.Use(Auth(config))
		router.GET("/test", func(c *gin.Context) {
			gotUserID := GetAuthUserID(c)
			gotEmail := GetAuthUserEmail(c)
			gotRole := GetAuthUserRole(c)

			assert.Equal(t, userID, gotUserID.String())
			assert.Equal(t, email, gotEmail)
			assert.Equal(t, role, gotRole)

			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(AuthUserRoleKey, "admin")
			c.Next()
		})
		router.Use(RequireRole("admin", "moderator"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InsufficientPermissions", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(AuthUserRoleKey, "user")
			c.Next()
		})
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("RoleNotFound", func(t *testing.T) {
		router := gin.New()
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestGetAuthUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ValidID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedID := uuid.New()
		c.Set(AuthUserIDKey, expectedID.String())

		result := GetAuthUserID(c)

		assert.Equal(t, expectedID, result)
	})

	t.Run("NotSet", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		result := GetAuthUserID(c)

		assert.Equal(t, uuid.Nil, result)
	})

	t.Run("InvalidType", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(AuthUserIDKey, 12345) // Wrong type

		result := GetAuthUserID(c)

		assert.Equal(t, uuid.Nil, result)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(AuthUserIDKey, "not-a-uuid")

		result := GetAuthUserID(c)

		assert.Equal(t, uuid.Nil, result)
	})
}

func TestGetAuthUserEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ValidEmail", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedEmail := "test@example.com"
		c.Set(AuthUserEmailKey, expectedEmail)

		result := GetAuthUserEmail(c)

		assert.Equal(t, expectedEmail, result)
	})

	t.Run("NotSet", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		result := GetAuthUserEmail(c)

		assert.Equal(t, "", result)
	})

	t.Run("InvalidType", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(AuthUserEmailKey, 12345)

		result := GetAuthUserEmail(c)

		assert.Equal(t, "", result)
	})
}

func TestGetAuthUserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ValidRole", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		expectedRole := "admin"
		c.Set(AuthUserRoleKey, expectedRole)

		result := GetAuthUserRole(c)

		assert.Equal(t, expectedRole, result)
	})

	t.Run("NotSet", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		result := GetAuthUserRole(c)

		assert.Equal(t, "", result)
	})

	t.Run("InvalidType", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(AuthUserRoleKey, 12345)

		result := GetAuthUserRole(c)

		assert.Equal(t, "", result)
	})
}

func TestMockTokenValidator(t *testing.T) {
	claims, err := MockTokenValidator("user-123")

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "user", claims.Role)
	assert.True(t, claims.Exp.After(time.Now()))
}

func TestAdminMockTokenValidator(t *testing.T) {
	claims, err := AdminMockTokenValidator("admin-456")

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "admin-456", claims.UserID)
	assert.Equal(t, "admin@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
	assert.True(t, claims.Exp.After(time.Now()))
}
