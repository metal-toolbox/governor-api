package v1alpha1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"go.uber.org/zap"
)

func (r *Router) mwContextInjectCorrelationID(c *gin.Context) {
	var correlationID string

	if cid := c.Request.Header.Get(events.GovernorEventCorrelationIDHeader); cid != "" {
		correlationID = cid
	} else {
		correlationID = uuid.New().String()
	}

	r.Logger.Debug("mwCorrelationID", zap.String("correlationID", correlationID))

	c.Request = c.Request.WithContext(events.InjectCorrelationID(
		c.Request.Context(),
		correlationID,
	))
}
