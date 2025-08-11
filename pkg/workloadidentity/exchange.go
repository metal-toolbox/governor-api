package workloadidentity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// SubjectTokenType is identifier that describes the token
// https://datatracker.ietf.org/doc/html/rfc8693#section-3
type SubjectTokenType string

const (
	// SubjectTokenTypeAccessToken indicates that the token is an OAuth 2.0
	// access token issued by the given authorization server.
	SubjectTokenTypeAccessToken SubjectTokenType = "urn:ietf:params:oauth:token-type:access_token"
	// SubjectTokenTypeIDToken indicates that the token is an ID Token as
	// defined in Section 2 of [OpenID.Core](https://openid.net/specs/openid-connect-core-1_0.html).
	SubjectTokenTypeIDToken SubjectTokenType = "urn:ietf:params:oauth:token-type:id_token"
	// SubjectTokenTypeRefreshToken indicates that the token is an OAuth 2.0
	// refresh token issued by the given authorization server.
	SubjectTokenTypeRefreshToken SubjectTokenType = "urn:ietf:params:oauth:token-type:refresh_token"
	// SubjectTokenTypeSAML1 indicates that the token is a base64url-encoded
	// SAML 1.1 [OASIS.saml-core-1.1] assertion.
	SubjectTokenTypeSAML1 SubjectTokenType = "urn:ietf:params:oauth:token-type:saml1"
	// SubjectTokenTypeSAML2 indicates that the token is a base64url-encoded
	// SAML 2.0 [OASIS.saml-core-2.0-os](https://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf)
	// assertion.
	SubjectTokenTypeSAML2 SubjectTokenType = "urn:ietf:params:oauth:token-type:saml2"
)

const (
	// grantType is the grant type for token exchange.
	grantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	// DefaultSubjectTokenType is the default subject token type for token exchange.
	DefaultSubjectTokenType = SubjectTokenTypeIDToken
)

// TokenExchangeSuccessfulResponse is the successful response for an RFC 8693
// request
// https://datatracker.ietf.org/doc/html/rfc8693#section-2.2.1
type TokenExchangeSuccessfulResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	Scope           string `json:"scope,omitempty"`
}

// TokenExchangeErrorResponse is the error response for an RFC 8693
// request
// https://datatracker.ietf.org/doc/html/rfc8693#section-2.2.2
// https://www.rfc-editor.org/rfc/rfc6749#section-5.2
type TokenExchangeErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// Token returns a new OAuth2 token for the workload identity.
func (w *WorkloadTokenSource) Token() (*oauth2.Token, error) {
	if w.token != nil && time.Until(w.token.Expiry) > w.tokenReuseExpiry {
		return w.token, nil
	}

	subjectToken, err := w.subjectTokenFn(w.ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting subject token: %w", err)
	}

	s := strings.TrimSpace(subjectToken.AccessToken)

	form := url.Values{}

	form.Add("grant_type", grantType)
	form.Add("subject_token_type", string(w.subjectTokenType))
	form.Add("subject_token", s)

	if len(w.scopes) > 0 {
		form.Add("scope", strings.Join(w.scopes, " "))
	}

	if w.audience != "" {
		form.Add("audience", w.audience)
	}

	req, err := http.NewRequestWithContext(w.ctx, http.MethodPost, w.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var errorResponse TokenExchangeErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return nil, fmt.Errorf(
				"%w: exchange failed with %s", ErrExchangingToken, resp.Status,
			)
		}

		var errmsg string

		if errorResponse.ErrorDescription != "" {
			errmsg = fmt.Sprintf("%s: %s", errorResponse.Error, errorResponse.ErrorDescription)
		} else {
			errmsg = errorResponse.Error
		}

		return nil, fmt.Errorf("%w: %s", ErrExchangingToken, errmsg)
	}

	var successResponse TokenExchangeSuccessfulResponse
	if err := json.NewDecoder(resp.Body).Decode(&successResponse); err != nil {
		return nil, fmt.Errorf("error decoding success response: %w", err)
	}

	t, _, err := jwt.NewParser().ParseUnverified(successResponse.AccessToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("error parsing access token: %w", err)
	}

	exp, err := t.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("error getting expiration time: %w", err)
	}

	w.token = &oauth2.Token{
		AccessToken: successResponse.AccessToken,
		TokenType:   successResponse.TokenType,
		Expiry:      exp.Time,
	}

	return w.token, nil
}
