package auth

import (
	"io"

	"github.com/metal-toolbox/hollow-toolbox/ginauth"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/auth/authz"
	"github.com/metal-toolbox/governor-api/internal/auth/cedar"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

// MultiTokenMiddlewareFromConfigs builds a MultiTokenMiddleware from the given
// auth providers. Each provider contributes exactly one verifier: a Cedar
// verifier when its cedar block is enabled (authenticates against that issuer,
// then authorizes via the sidecar), otherwise the plain scope-based ginjwt
// verifier. The middleware runs all verifiers concurrently and passes on the
// first success, so a token from a Cedar-authorized issuer succeeds via its
// Cedar verifier while tokens from scope-gated issuers succeed via their own
// ginjwt verifiers.
func MultiTokenMiddlewareFromConfigs(
	auths []configs.Auth,
	logger *zap.Logger,
	auditWriter io.Writer,
) (*ginauth.MultiTokenMiddleware, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	mtm, err := ginauth.NewMultiTokenMiddleware()
	if err != nil {
		return nil, err
	}

	for i := range auths {
		a := auths[i]

		mw, err := verifierFromAuth(a, logger, auditWriter)
		if err != nil {
			return nil, err
		}

		if a.Cedar.Enabled {
			logger.Info("cedar authorization enabled for issuer",
				zap.String("issuer", a.Issuer),
				zap.String("url", a.Cedar.URL),
				zap.Duration("timeout", a.Cedar.TimeoutOrDefault()),
			)
		}

		if err := mtm.Add(mw); err != nil {
			return nil, err
		}
	}

	return mtm, nil
}

// verifierFromAuth returns the single verifier for one provider: a Cedar
// verifier when cedar is enabled, otherwise the plain ginjwt verifier.
func verifierFromAuth(a configs.Auth, logger *zap.Logger, aw io.Writer) (ginauth.GenericAuthMiddleware, error) {
	jwtMW, err := ginjwt.NewAuthMiddleware(a.AuthConfig)
	if err != nil {
		return nil, err
	}

	if !a.Cedar.Enabled {
		return jwtMW, nil
	}

	decider := cedar.NewDecider(
		a.Cedar.URL,
		a.Cedar.TimeoutOrDefault(),
		cedar.WithLogger(logger),
		cedar.WithAuditWriter(aw),
	)

	return authz.NewVerifier(jwtMW, decider), nil
}
