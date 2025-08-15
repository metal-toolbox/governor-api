package workloadidentity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var errSubjectToken = errors.New("subject token error")

type WorkloadIdentityTestSuite struct {
	suite.Suite

	server     *httptest.Server
	tempDir    string
	tokenPath  string
	validJWT   string
	expiredJWT string
}

func (s *WorkloadIdentityTestSuite) SetupSuite() {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "workloadidentity_test")
	s.Require().NoError(err)
	s.tempDir = tempDir
	s.tokenPath = filepath.Join(tempDir, "token")

	// Create valid JWT token for testing
	s.validJWT = s.createTestJWT(time.Now().Add(time.Hour))
	s.expiredJWT = s.createTestJWT(time.Now().Add(-time.Hour))

	// Write valid token to file
	err = os.WriteFile(s.tokenPath, []byte(s.validJWT), 0o600)
	s.Require().NoError(err)
}

func (s *WorkloadIdentityTestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}

	os.RemoveAll(s.tempDir)
}

func (s *WorkloadIdentityTestSuite) SetupTest() {
	// Reset server for each test
	if s.server != nil {
		s.server.Close()
	}
}

func (s *WorkloadIdentityTestSuite) createTestJWT(exp time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "test-subject",
		"aud": "test-audience",
		"iss": "test-issuer",
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte("test-secret"))
	s.Require().NoError(err)

	return tokenString
}

func (s *WorkloadIdentityTestSuite) createMockServer(handler http.HandlerFunc) {
	s.server = httptest.NewServer(handler)
}

func (s *WorkloadIdentityTestSuite) TestNewWorkloadTokenSource() {
	ctx := context.Background()
	tokenURL := "https://example.com/token"

	tests := []struct {
		name     string
		opts     []Option
		validate func(*WorkloadTokenSource)
	}{
		{
			name: "default configuration",
			opts: nil,
			validate: func(w *WorkloadTokenSource) {
				assert.Equal(s.T(), tokenURL, w.tokenURL)
				assert.Equal(s.T(), defaultRequestTimeout, w.requestTimeout)
				assert.Equal(s.T(), defaultReuseExpiry, w.tokenReuseExpiry)
				assert.Equal(s.T(), DefaultSubjectTokenType, w.subjectTokenType)
				assert.NotNil(s.T(), w.subjectTokenFn)
				assert.NotNil(s.T(), w.httpClient)
				assert.NotNil(s.T(), w.logger)
				assert.NotNil(s.T(), w.tracer)
			},
		},
		{
			name: "with custom options",
			opts: []Option{
				WithScopes("scope1", "scope2"),
				WithAudience("test-audience"),
				WithTokenReuseExpiry(time.Minute),
				WithRequestTimeout(time.Second * 30),
				WithLogger(zap.NewNop()),
			},
			validate: func(w *WorkloadTokenSource) {
				assert.Equal(s.T(), []string{"scope1", "scope2"}, w.scopes)
				assert.Equal(s.T(), "test-audience", w.audience)
				assert.Equal(s.T(), time.Minute, w.tokenReuseExpiry)
				assert.Equal(s.T(), time.Second*30, w.requestTimeout)
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(_ *testing.T) {
			ts := NewWorkloadTokenSource(ctx, tokenURL, tt.opts...)
			tt.validate(ts)
		})
	}
}

func (s *WorkloadIdentityTestSuite) TestNewTokenSource() {
	ctx := context.Background()
	tokenURL := "https://example.com/token"

	tokenSource := NewTokenSource(ctx, tokenURL)
	assert.NotNil(s.T(), tokenSource)

	// The NewTokenSource function returns a TokenSource interface,
	// but we can't easily test the concrete type since oauth2.ReuseTokenSource
	// returns an unexported type. We'll just verify it's not nil and can be used.
	assert.Implements(s.T(), (*oauth2.TokenSource)(nil), tokenSource)
}

func (s *WorkloadIdentityTestSuite) TestKubeServiceAccountTokenFn() {
	ctx := context.Background()
	tokenURL := "https://example.com/token"

	tests := []struct {
		name           string
		tokenPath      string
		tokenContent   string
		expectedError  string
		validateResult func(*oauth2.Token)
	}{
		{
			name:         "valid token file",
			tokenPath:    s.tokenPath,
			tokenContent: s.validJWT,
			validateResult: func(token *oauth2.Token) {
				assert.Equal(s.T(), s.validJWT, token.AccessToken)
				assert.Equal(s.T(), "Bearer", token.TokenType)
				assert.True(s.T(), token.Expiry.After(time.Now()))
			},
		},
		{
			name:          "nonexistent token file",
			tokenPath:     "/nonexistent/path",
			expectedError: "error reading kube token file",
		},
		{
			name:          "invalid JWT token",
			tokenPath:     filepath.Join(s.tempDir, "invalid_token"),
			tokenContent:  "invalid-jwt-token",
			expectedError: "error parsing kube token",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			if tt.tokenContent != "" {
				err := os.WriteFile(tt.tokenPath, []byte(tt.tokenContent), 0o600)
				s.Require().NoError(err)
			}

			ts := NewWorkloadTokenSource(ctx, tokenURL, WithKubeSubjectToken(tt.tokenPath, SubjectTokenTypeIDToken))
			token, err := ts.subjectTokenFn(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)

				if tt.validateResult != nil {
					tt.validateResult(token)
				}
			}
		})
	}
}

func (s *WorkloadIdentityTestSuite) TestTokenExchange() {
	ctx := context.Background()

	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		options        []Option
		expectedError  string
		validateToken  func(*oauth2.Token)
	}{
		{
			name: "successful token exchange",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(s.T(), "POST", r.Method)
				assert.Equal(s.T(), "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

				// Parse form
				err := r.ParseForm()
				assert.NoError(s.T(), err)
				assert.Equal(s.T(), grantType, r.Form.Get("grant_type"))
				assert.Equal(s.T(), string(SubjectTokenTypeIDToken), r.Form.Get("subject_token_type"))
				assert.NotEmpty(s.T(), r.Form.Get("subject_token"))

				// Create response JWT
				responseJWT := s.createTestJWT(time.Now().Add(time.Hour))
				response := TokenExchangeSuccessfulResponse{
					AccessToken:     responseJWT,
					IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
					TokenType:       "Bearer",
					ExpiresIn:       3600,
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			},
			validateToken: func(token *oauth2.Token) {
				assert.NotEmpty(s.T(), token.AccessToken)
				assert.Equal(s.T(), "Bearer", token.TokenType)
				assert.True(s.T(), token.Expiry.After(time.Now()))
			},
		},
		{
			name: "token exchange with scopes and audience",
			options: []Option{
				WithScopes("read", "write"),
				WithAudience("test-audience"),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				assert.NoError(s.T(), err)
				assert.Equal(s.T(), "read write", r.Form.Get("scope"))
				assert.Equal(s.T(), "test-audience", r.Form.Get("audience"))

				responseJWT := s.createTestJWT(time.Now().Add(time.Hour))
				response := TokenExchangeSuccessfulResponse{
					AccessToken:     responseJWT,
					IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
					TokenType:       "Bearer",
					ExpiresIn:       3600,
					Scope:           "read write",
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			},
			validateToken: func(token *oauth2.Token) {
				assert.NotEmpty(s.T(), token.AccessToken)
				assert.Equal(s.T(), "Bearer", token.TokenType)
			},
		},
		{
			name: "server error response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				response := TokenExchangeErrorResponse{
					Error:            "invalid_request",
					ErrorDescription: "The request is missing a required parameter",
				}
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedError: "invalid_request: The request is missing a required parameter",
		},
		{
			name: "server error without description",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				response := TokenExchangeErrorResponse{
					Error: "invalid_grant",
				}
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedError: "invalid_grant",
		},
		{
			name: "server error without error field",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal Server Error"))
			},
			expectedError: "exchange failed with 500 Internal Server Error",
		},
		{
			name: "server error with empty error response JSON",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				response := TokenExchangeErrorResponse{} // Empty error field
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedError: "no additional information was provided by IDP",
		},
		{
			name: "invalid response JSON",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("invalid json"))
			},
			expectedError: "error decoding success response",
		},
		{
			name: "invalid access token JWT",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				response := TokenExchangeSuccessfulResponse{
					AccessToken:     "invalid.jwt.token",
					IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
					TokenType:       "Bearer",
					ExpiresIn:       3600,
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedError: "error parsing access token",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			s.createMockServer(tt.serverResponse)

			options := []Option{
				WithKubeSubjectToken(s.tokenPath, SubjectTokenTypeIDToken),
			}
			options = append(options, tt.options...)

			ts := NewWorkloadTokenSource(ctx, s.server.URL, options...)
			token, err := ts.Token()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, token)

				if tt.validateToken != nil {
					tt.validateToken(token)
				}
			}
		})
	}
}

func (s *WorkloadIdentityTestSuite) TestTokenReuse() {
	ctx := context.Background()

	callCount := 0

	s.createMockServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		responseJWT := s.createTestJWT(time.Now().Add(time.Hour))
		response := TokenExchangeSuccessfulResponse{
			AccessToken:     responseJWT,
			IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
			TokenType:       "Bearer",
			ExpiresIn:       3600,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	ts := NewWorkloadTokenSource(ctx, s.server.URL,
		WithKubeSubjectToken(s.tokenPath, SubjectTokenTypeIDToken),
		WithTokenReuseExpiry(time.Second*30),
	)

	// First call should make HTTP request
	token1, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), token1)
	assert.Equal(s.T(), 1, callCount)

	// Second call should reuse token
	token2, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), token2)
	assert.Equal(s.T(), token1.AccessToken, token2.AccessToken)
	assert.Equal(s.T(), 1, callCount) // No additional HTTP call
}

func (s *WorkloadIdentityTestSuite) TestSubjectTokenReuse() {
	ctx := context.Background()

	callCount := 0

	s.createMockServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		responseJWT := s.createTestJWT(time.Now().Add(time.Hour))
		response := TokenExchangeSuccessfulResponse{
			AccessToken:     responseJWT,
			IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
			TokenType:       "Bearer",
			ExpiresIn:       3600,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	subjectTokenCallCount := 0
	customSubjectTokenFn := func(_ context.Context) (*oauth2.Token, error) {
		subjectTokenCallCount++

		return &oauth2.Token{
			AccessToken: s.validJWT,
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(time.Hour),
		}, nil
	}

	ts := NewWorkloadTokenSource(ctx, s.server.URL,
		WithSubjectTokenFn(customSubjectTokenFn, SubjectTokenTypeIDToken),
		WithTokenReuseExpiry(time.Second*30),
	)

	// First call
	_, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, subjectTokenCallCount)
	assert.Equal(s.T(), 1, callCount)

	// Second call should reuse subject token
	_, err = ts.Token()
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, subjectTokenCallCount) // Subject token reused
	assert.Equal(s.T(), 1, callCount)             // Access token reused
}

func (s *WorkloadIdentityTestSuite) TestTokenExpirationHandling() {
	ctx := context.Background()

	callCount := 0

	s.createMockServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// Create token that expires in different times based on call count
		var expiry time.Time
		if callCount == 1 {
			// First token expires very soon (within token reuse expiry)
			expiry = time.Now().Add(time.Second * 10)
		} else {
			// Second token has longer expiry
			expiry = time.Now().Add(time.Hour)
		}

		responseJWT := s.createTestJWT(expiry)
		response := TokenExchangeSuccessfulResponse{
			AccessToken:     responseJWT,
			IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
			TokenType:       "Bearer",
			ExpiresIn:       int(time.Until(expiry).Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	ts := NewWorkloadTokenSource(ctx, s.server.URL,
		WithKubeSubjectToken(s.tokenPath, SubjectTokenTypeIDToken),
		WithTokenReuseExpiry(time.Second*30), // Token reuse expiry is 30 seconds
	)

	// First call should make HTTP request and get a token that expires soon
	token1, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), token1)
	assert.Equal(s.T(), 1, callCount)

	// Second call should detect the token is close to expiry and request a new one
	token2, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), token2)
	assert.Equal(s.T(), 2, callCount)                              // New HTTP call made because token was close to expiry
	assert.NotEqual(s.T(), token1.AccessToken, token2.AccessToken) // Should be different tokens

	// Third call should reuse the second token since it has longer expiry
	token3, err := ts.Token()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), token3)
	assert.Equal(s.T(), 2, callCount)                           // No new HTTP call
	assert.Equal(s.T(), token2.AccessToken, token3.AccessToken) // Should be same token
}

func (s *WorkloadIdentityTestSuite) TestSubjectTokenError() {
	ctx := context.Background()

	errorSubjectTokenFn := func(_ context.Context) (*oauth2.Token, error) {
		return nil, errSubjectToken
	}

	ts := NewWorkloadTokenSource(ctx, "https://example.com/token",
		WithSubjectTokenFn(errorSubjectTokenFn, SubjectTokenTypeIDToken),
	)

	token, err := ts.Token()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "error getting subject token")
	assert.Nil(s.T(), token)
}

func (s *WorkloadIdentityTestSuite) TestHTTPClientError() {
	ctx := context.Background()

	// Use invalid URL to trigger HTTP client error
	ts := NewWorkloadTokenSource(ctx, "http://invalid-url-that-does-not-exist.local",
		WithKubeSubjectToken(s.tokenPath, SubjectTokenTypeIDToken),
		WithRequestTimeout(time.Millisecond*100),
	)

	token, err := ts.Token()
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "error sending request")
	assert.Nil(s.T(), token)
}

func (s *WorkloadIdentityTestSuite) TestOptions() {
	ctx := context.Background()
	tokenURL := "https://example.com/token"

	s.T().Run("WithKubeSubjectToken", func(t *testing.T) {
		ts := NewWorkloadTokenSource(ctx, tokenURL,
			WithKubeSubjectToken("/custom/path", SubjectTokenTypeAccessToken),
		)
		assert.Equal(t, SubjectTokenTypeAccessToken, ts.subjectTokenType)
	})

	s.T().Run("WithLogger", func(t *testing.T) {
		logger := zap.NewNop()
		ts := NewWorkloadTokenSource(ctx, tokenURL, WithLogger(logger))
		assert.Equal(t, logger, ts.logger)
	})

	s.T().Run("WithHTTPClient", func(t *testing.T) {
		client := &http.Client{Timeout: time.Second * 10}
		ts := NewWorkloadTokenSource(ctx, tokenURL, WithHTTPClient(client))
		assert.Equal(t, client, ts.httpClient)
	})

	s.T().Run("WithTracer", func(t *testing.T) {
		tracer := otel.GetTracerProvider().Tracer("test")
		ts := NewWorkloadTokenSource(ctx, tokenURL, WithTracer(tracer))
		assert.Equal(t, tracer, ts.tracer)
	})
}

func (s *WorkloadIdentityTestSuite) TestSubjectTokenTypes() {
	tests := []struct {
		name      string
		tokenType SubjectTokenType
		expected  string
	}{
		{"AccessToken", SubjectTokenTypeAccessToken, "urn:ietf:params:oauth:token-type:access_token"},
		{"IDToken", SubjectTokenTypeIDToken, "urn:ietf:params:oauth:token-type:id_token"},
		{"RefreshToken", SubjectTokenTypeRefreshToken, "urn:ietf:params:oauth:token-type:refresh_token"},
		{"SAML1", SubjectTokenTypeSAML1, "urn:ietf:params:oauth:token-type:saml1"},
		{"SAML2", SubjectTokenTypeSAML2, "urn:ietf:params:oauth:token-type:saml2"},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.tokenType))
		})
	}

	s.T().Run("DefaultSubjectTokenType", func(_ *testing.T) {
		assert.Equal(s.T(), SubjectTokenTypeIDToken, DefaultSubjectTokenType)
	})
}

func (s *WorkloadIdentityTestSuite) TestTokenExchangeResponses() {
	s.T().Run("TokenExchangeSuccessfulResponse", func(t *testing.T) {
		response := TokenExchangeSuccessfulResponse{
			AccessToken:     "access_token_value",
			IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
			TokenType:       "Bearer",
			ExpiresIn:       3600,
			Scope:           "read write",
		}

		data, err := json.Marshal(response)
		assert.NoError(t, err)

		var unmarshaled TokenExchangeSuccessfulResponse
		err = json.Unmarshal(data, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, response, unmarshaled)
	})

	s.T().Run("TokenExchangeErrorResponse", func(t *testing.T) {
		response := TokenExchangeErrorResponse{
			Error:            "invalid_request",
			ErrorDescription: "The request is missing a required parameter",
			ErrorURI:         "https://example.com/error",
		}

		data, err := json.Marshal(response)
		assert.NoError(t, err)

		var unmarshaled TokenExchangeErrorResponse
		err = json.Unmarshal(data, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, response, unmarshaled)
	})
}

func (s *WorkloadIdentityTestSuite) TestConstants() {
	assert.Equal(s.T(), "urn:ietf:params:oauth:grant-type:token-exchange", grantType)
	assert.Equal(s.T(), 30*time.Second, defaultReuseExpiry)
	assert.Equal(s.T(), 15*time.Second, defaultRequestTimeout)
	assert.Equal(s.T(), "/var/run/secrets/kubernetes.io/serviceaccount/token", defaultKubeServiceAccountPath)
}

func (s *WorkloadIdentityTestSuite) TestValidateSubjectTokenType() {
	tests := []struct {
		name          string
		tokenType     string
		expectedError bool
	}{
		{
			name:          "valid access token type",
			tokenType:     string(SubjectTokenTypeAccessToken),
			expectedError: false,
		},
		{
			name:          "valid id token type",
			tokenType:     string(SubjectTokenTypeIDToken),
			expectedError: false,
		},
		{
			name:          "valid refresh token type",
			tokenType:     string(SubjectTokenTypeRefreshToken),
			expectedError: false,
		},
		{
			name:          "valid saml1 token type",
			tokenType:     string(SubjectTokenTypeSAML1),
			expectedError: false,
		},
		{
			name:          "valid saml2 token type",
			tokenType:     string(SubjectTokenTypeSAML2),
			expectedError: false,
		},
		{
			name:          "invalid token type",
			tokenType:     "invalid-token-type",
			expectedError: true,
		},
		{
			name:          "empty token type",
			tokenType:     "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			_, err := NewSubjectTokenTypeFromString(tt.tokenType)
			if tt.expectedError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidSubjectTokenType)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkloadIdentityTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadIdentityTestSuite))
}
