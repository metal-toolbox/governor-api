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
