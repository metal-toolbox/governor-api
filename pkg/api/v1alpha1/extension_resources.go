package v1alpha1

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
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

	ownerID := ""

	if req.Metadata.OwnerRef.Kind != "" {
		if req.Metadata.OwnerRef.Kind != ExtensionResourceOwnerKindGroup {
			span.SetStatus(codes.Error, "invalid owner kind")
			sendError(c, http.StatusBadRequest, "owner_ref.kind must be 'group' with system extension resources")

			return
		}

		if req.Metadata.OwnerRef.ID != "" {
			ownerID = req.Metadata.OwnerRef.ID
		}
	}

	res := createSystemExtensionResource(c, r.DB, r.EventBus, extension, erd, req.Spec, ownerID)
	if res == nil {
		return
	}

	resp := &ExtensionResource{
		Extension: req.Extension,
		Kind:      req.Kind,
		Version:   req.Version,
		Spec:      req.Spec,
		Metadata: ExtensionResourceMetadata{
			OwnerRef:        req.Metadata.OwnerRef,
			CreatedAt:       res.CreatedAt.Format(time.RFC3339),
			ID:              res.ID,
			ResourceVersion: res.ResourceVersion,
		},
		Status: ExtensionResourceStatus{
			UpdatedAt: res.UpdatedAt.Format(time.RFC3339),
		},
	}

	resp.Status.Messages = make([]json.RawMessage, len(res.Messages))
	for i, msg := range res.Messages {
		resp.Status.Messages[i] = json.RawMessage(msg)
	}

	c.JSON(http.StatusCreated, resp)
}

func (r *Router) updateExtensionResource(c *gin.Context) {
	_, span := tracer.Start(c.Request.Context(), "updateExtensionResource")
	defer span.End()
	defer c.Request.Body.Close()

	req := getCtxExtensionResource(c)

	rid := c.Param("resource-id")
	if rid == "" {
		span.SetStatus(codes.Error, "missing resource ID in request body")
		sendError(c, http.StatusBadRequest, "metadata.id is required for update")

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
		attribute.String("resource.id", rid),
	)

	if erd.Scope == ExtensionResourceDefinitionScopeUser.String() {
		span.SetStatus(codes.Error, "unimplemented")
		sendError(c, http.StatusNotImplemented, "user-scoped extension resources are not yet supported with this API")

		return
	}

	var msgs []string = nil
	if len(req.Status.Messages) > 0 {
		msgs = make([]string, 0, len(req.Status.Messages))
		for _, m := range req.Status.Messages {
			msgs = append(msgs, string(m))
		}
	}

	var rv *int64 = nil
	if req.Metadata.ResourceVersion != 0 {
		rv = &req.Metadata.ResourceVersion
		r.Logger.Debug("current resource version", zap.Int64("resource_version", *rv))
	}

	res := updateSystemExtensionResource(c, r.DB, r.EventBus, extension, erd, rid, req.Spec, msgs, rv)
	if res == nil {
		return
	}

	resp := &ExtensionResource{}

	resp.Extension = req.Extension
	resp.Kind = req.Kind
	resp.Version = req.Version
	resp.Metadata.CreatedAt = res.CreatedAt.Format(time.RFC3339)
	resp.Metadata.ID = res.ID
	resp.Metadata.ResourceVersion = res.ResourceVersion

	if res.OwnerID.Valid && res.OwnerID.String != "" {
		resp.Metadata.OwnerRef = ExtensionResourceMetadataOwnerRef{
			Kind: ExtensionResourceOwnerKindGroup,
			ID:   res.OwnerID.String,
		}
	}

	resp.Spec = json.RawMessage(res.Resource)
	resp.Status.UpdatedAt = res.UpdatedAt.Format(time.RFC3339)

	resp.Status.Messages = make([]json.RawMessage, len(res.Messages))
	for i, msg := range res.Messages {
		resp.Status.Messages[i] = json.RawMessage(msg)
	}

	c.JSON(http.StatusAccepted, resp)
}
