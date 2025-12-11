package v1alpha1

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type ExtensionResourceMetadata struct {
	OwnerRef        ExtensionResourceMetadataOwnerRef `json:"owner_ref,omitempty"`
	ID              string                            `json:"id,omitempty"`
	ResourceVersion int64                             `json:"resource_version,omitempty"`
	CreatedAt       string                            `json:"created_at,omitempty"`
}

type ExtensionResourceOwnerKind string

const (
	ExtensionResourceOwnerKindGroup ExtensionResourceOwnerKind = "group"
	ExtensionResourceOwnerKindUser  ExtensionResourceOwnerKind = "user"
)

type ExtensionResourceMetadataOwnerRef struct {
	Kind ExtensionResourceOwnerKind `json:"kind"`
	ID   string                     `json:"id"`
}

type ExtensionResourceStatus struct {
	State     string            `json:"state,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty"`
	Messages  []json.RawMessage `json:"messages,omitempty"`
}

type ExtensionResource struct {
	Extension string `json:"extension"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`

	Metadata ExtensionResourceMetadata `json:"metadata"`
	Spec     json.RawMessage           `json:"spec"`

	Status ExtensionResourceStatus `json:"status,omitempty"`
}

type APIStatusMessage struct {
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func (r *Router) createExtensionResource(c *gin.Context) {
	_, span := tracer.Start(c.Request.Context(), "createExtensionResource")
	defer span.End()
	defer c.Request.Body.Close()

	req := getCtxExtensionResource(c)

	if req == nil {
		if err := json.NewDecoder(c.Request.Body).Decode(req); err != nil {
			span.SetStatus(codes.Error, "invalid request body")
			span.RecordError(err)
			sendError(c, http.StatusBadRequest, err.Error())

			return
		}
	}

	if req.Extension == "" || req.Kind == "" || req.Version == "" {
		span.SetStatus(codes.Error, "missing required fields in request body")
		sendError(c, http.StatusBadRequest, "missing required fields in request body")

		return
	}

	// find ERD
	extension, erd, err := findERDForExtensionResource(
		c, r.DB,
		req.Extension, req.Kind, req.Version,
		findERDUseSlugSingular(),
	)
	if err != nil {
		if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, err.Error())
			return
		}

		sendError(c, http.StatusBadRequest, err.Error())

		return
	}

	span.SetAttributes(
		attribute.String("extension", req.Extension),
		attribute.String("kind", req.Kind),
		attribute.String("version", req.Version),
	)

	if erd.Scope == ExtensionResourceDefinitionScopeUser.String() {
		span.SetStatus(codes.Error, "unimplemented")
		sendError(c, http.StatusNotImplemented, "user-scoped extension resources are not yet supported with this API")

		return
	}

	if req.Metadata.OwnerRef.Kind != ExtensionResourceOwnerKindGroup {
		span.SetStatus(codes.Error, "invalid owner kind")
		sendError(c, http.StatusBadRequest, "owner_ref.kind must be 'group' with system extension resources")

		return
	}

	ownerID := ""
	if req.Metadata.OwnerRef.ID != "" {
		ownerID = req.Metadata.OwnerRef.ID
	}

	res := createSystemExtensionResourceCore(c, r.DB, r.EventBus, extension, erd, req.Spec, ownerID)

	resp := &ExtensionResource{}
	*resp = *req

	resp.Metadata.CreatedAt = res.CreatedAt.Format(time.RFC3339)
	resp.Metadata.ID = res.ID
	resp.Metadata.ResourceVersion = res.ResourceVersion
	resp.Status.UpdatedAt = res.UpdatedAt.Format(time.RFC3339)

	resp.Status.Messages = make([]json.RawMessage, len(res.Messages))
	for i, msg := range res.Messages {
		resp.Status.Messages[i] = json.RawMessage(msg)
	}

	c.JSON(http.StatusCreated, resp)
}
