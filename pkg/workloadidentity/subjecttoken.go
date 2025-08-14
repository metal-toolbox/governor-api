package workloadidentity

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
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

// NewSubjectTokenTypeFromString creates a new SubjectTokenType from a string.
func NewSubjectTokenTypeFromString(in string) (SubjectTokenType, error) {
	switch in {
	case string(SubjectTokenTypeAccessToken):
		return SubjectTokenTypeAccessToken, nil
	case string(SubjectTokenTypeIDToken):
		return SubjectTokenTypeIDToken, nil
	case string(SubjectTokenTypeRefreshToken):
		return SubjectTokenTypeRefreshToken, nil
	case string(SubjectTokenTypeSAML1):
		return SubjectTokenTypeSAML1, nil
	case string(SubjectTokenTypeSAML2):
		return SubjectTokenTypeSAML2, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidSubjectTokenType, in)
	}
}

const defaultKubeServiceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// kubeServiceAccountTokenFn returns a function that retrieves the Kubernetes service account token.
func (w *WorkloadTokenSource) kubeServiceAccountTokenFn(path string) SubjectTokenFn {
	return func(ctx context.Context) (*oauth2.Token, error) {
		_, span := w.tracer.Start(ctx, "kubeServiceAccountTokenFn")
		defer span.End()

		tokenData, err := os.ReadFile(path)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error reading kube token file")

			return nil, fmt.Errorf("error reading kube token file: %w", err)
		}

		tokenstr := strings.TrimSpace(string(tokenData))

		token, _, err := jwt.NewParser().ParseUnverified(tokenstr, jwt.MapClaims{})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error parsing kube token")

			return nil, fmt.Errorf("error parsing kube token: %w", err)
		}

		exp, err := token.Claims.GetExpirationTime()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error getting expiration time")

			return nil, fmt.Errorf("error getting expiration time: %w", err)
		}

		expiryTime := time.Time{}
		if exp != nil {
			expiryTime = exp.Time
		}

		w.logger.Debug(
			"subject token",
			zap.Time("exp", expiryTime),
			zap.Any("claims", token.Claims),
		)

		return &oauth2.Token{
			AccessToken: tokenstr,
			TokenType:   "Bearer",
			Expiry:      expiryTime,
		}, nil
	}
}
