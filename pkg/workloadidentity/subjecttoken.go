package workloadidentity

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const defaultKubeServiceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// kubeServiceAccountTokenFn returns a function that retrieves the Kubernetes service account token.
func kubeServiceAccountTokenFn(path string, logger *zap.Logger) SubjectTokenFn {
	if logger == nil {
		logger = zap.NewNop()
	}

	return func(_ context.Context) (*oauth2.Token, error) {
		tokenData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("error reading kube token file: %w", err)
		}

		tokenstr := string(tokenData)

		token, _, err := jwt.NewParser().ParseUnverified(tokenstr, jwt.MapClaims{})
		if err != nil {
			return nil, fmt.Errorf("error parsing kube token: %w", err)
		}

		exp, err := token.Claims.GetExpirationTime()
		if err != nil {
			return nil, fmt.Errorf("error getting expiration time: %w", err)
		}

		expiryTime := time.Time{}
		if exp != nil {
			expiryTime = exp.Time
		}

		logger.Debug(
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
