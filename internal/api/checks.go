package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// livenessCheck ensures that the server is up and responding
func (s *Server) livenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "UP",
	})
}

// readinessCheck ensures that the server is up and that we are able to process
// requests. It will check that the database is up and that we can reach all
// configured auth providers.
func (s *Server) readinessCheck(c *gin.Context) {
	if err := s.DB.PingContext(c.Request.Context()); err != nil {
		s.Conf.Logger.Error("readiness check db ping failed", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "DOWN",
		})

		return
	}

	for _, authConf := range s.Conf.AuthConf {
		if err := urlPingContext(c.Request.Context(), authConf.JWKSURI); err != nil {
			s.Conf.Logger.Error("readiness check auth jwksuri ping failed", zap.String("jwksuri", authConf.JWKSURI), zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "DOWN",
			})

			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "UP",
	})
}

func urlPingContext(ctx context.Context, url string) error {
	const timeout = 5 * time.Second

	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("bad response: " + resp.Status) //nolint:err113
	}

	return nil
}
