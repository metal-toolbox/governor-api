package workloadidentity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
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
	ctx, span := w.tracer.Start(w.ctx, "WorkloadTokenSource.Token", trace.WithNewRoot())
	defer span.End()

	if w.token != nil && time.Until(w.token.Expiry) > w.tokenReuseExpiry {
		span.AddEvent("token reused")
		span.SetAttributes(attribute.Int64("exp", w.token.Expiry.Unix()))

		return w.token, nil
	}

	var subjectToken string
	if w.subjectToken != nil && time.Until(w.subjectToken.Expiry) > w.tokenReuseExpiry {
		subjectToken = w.subjectToken.AccessToken
	} else {
		st, err := w.subjectTokenFn(w.ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error getting subject token")

			return nil, fmt.Errorf("error getting subject token: %w", err)
		}

		w.subjectToken = st
		subjectToken = st.AccessToken
	}

	form := url.Values{}

	form.Add("grant_type", grantType)
	form.Add("subject_token_type", string(w.subjectTokenType))
	form.Add("subject_token", subjectToken)

	if len(w.scopes) > 0 {
		form.Add("scope", strings.Join(w.scopes, " "))
	}

	if w.audience != "" {
		form.Add("audience", w.audience)
	}

	ctx, cancel := context.WithTimeout(ctx, w.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error creating request")

		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error sending request")

		return nil, fmt.Errorf("error sending request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var errorResponse TokenExchangeErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error decoding response")

			return nil, fmt.Errorf(
				"%w: exchange failed with %s", ErrExchangingToken, resp.Status,
			)
		}

		var errmsg string

		switch {
		case errorResponse.Error == "":
			errmsg = "no additional information was provided by IDP"
		case errorResponse.ErrorDescription != "":
			errmsg = fmt.Sprintf("%s: %s", errorResponse.Error, errorResponse.ErrorDescription)
		default:
			errmsg = errorResponse.Error
		}

		err := fmt.Errorf("%w: %s", ErrExchangingToken, errmsg)

		span.RecordError(err)
		span.SetStatus(codes.Error, "error exchanging token")

		return nil, err
	}

	var successResponse TokenExchangeSuccessfulResponse
	if err := json.NewDecoder(resp.Body).Decode(&successResponse); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error decoding success response")

		return nil, fmt.Errorf("error decoding success response: %w", err)
	}

	t, _, err := jwt.NewParser().ParseUnverified(successResponse.AccessToken, jwt.MapClaims{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error parsing access token")

		return nil, fmt.Errorf("error parsing access token: %w", err)
	}

	exp, err := t.Claims.GetExpirationTime()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error getting expiration time")

		return nil, fmt.Errorf("error getting expiration time: %w", err)
	}

	w.logger.Debug("exchanged token", zap.Any("claims", t.Claims))

	w.token = &oauth2.Token{
		AccessToken: successResponse.AccessToken,
		TokenType:   successResponse.TokenType,
		Expiry:      exp.Time,
	}

	return w.token, nil
}
