package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	platform "github.com/influxdata/influxdb"
	platcontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/jsonweb"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
)

// AuthenticationHandler is a middleware for authenticating incoming requests.
type AuthenticationHandler struct {
	platform.HTTPErrorHandler
	Logger *zap.Logger

	AuthorizationService platform.AuthorizationService
	SessionService       platform.SessionService
	UserService          platform.UserService
	TokenParser          *jsonweb.TokenParser
	SessionRenewDisabled bool

	// This is only really used for it's lookup method the specific http
	// handler used to register routes does not matter.
	noAuthRouter *httprouter.Router

	Handler http.Handler
}

// NewAuthenticationHandler creates an authentication handler.
func NewAuthenticationHandler(h platform.HTTPErrorHandler) *AuthenticationHandler {
	return &AuthenticationHandler{
		Logger:           zap.NewNop(),
		HTTPErrorHandler: h,
		Handler:          http.DefaultServeMux,
		TokenParser:      jsonweb.NewTokenParser(jsonweb.EmptyKeyStore),
		noAuthRouter:     httprouter.New(),
	}
}

// RegisterNoAuthRoute excludes routes from needing authentication.
func (h *AuthenticationHandler) RegisterNoAuthRoute(method, path string) {
	// the handler specified here does not matter.
	h.noAuthRouter.HandlerFunc(method, path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
}

const (
	tokenAuthScheme   = "token"
	sessionAuthScheme = "session"
)

// ProbeAuthScheme probes the http request for the requests for token or cookie session.
func ProbeAuthScheme(r *http.Request) (string, error) {
	_, tokenErr := GetToken(r)
	_, sessErr := decodeCookieSession(r.Context(), r)

	if tokenErr != nil && sessErr != nil {
		return "", fmt.Errorf("token required")
	}

	if tokenErr == nil {
		return tokenAuthScheme, nil
	}

	return sessionAuthScheme, nil
}

func (h *AuthenticationHandler) unauthorized(ctx context.Context, w http.ResponseWriter, err error) {
	h.Logger.Info("unauthorized", zap.Error(err))
	UnauthorizedError(ctx, h, w)
}

// ServeHTTP extracts the session or token from the http request and places the resulting authorizer on the request context.
func (h *AuthenticationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler, _, _ := h.noAuthRouter.Lookup(r.Method, r.URL.Path); handler != nil {
		h.Handler.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()
	scheme, err := ProbeAuthScheme(r)
	if err != nil {
		h.unauthorized(ctx, w, err)
		return
	}

	var auth platform.Authorizer

	switch scheme {
	case tokenAuthScheme:
		auth, err = h.extractAuthorization(ctx, r)
		if err != nil {
			h.unauthorized(ctx, w, err)
			return
		}
	case sessionAuthScheme:
		auth, err = h.extractSession(ctx, r)
		if err != nil {
			h.unauthorized(ctx, w, err)
			return
		}
	default:
		h.unauthorized(ctx, w, err)
		return
	}

	// jwt based auth is permission based rather than identity based
	// and therefor has no associated user. if the user ID is invalid
	// disregard the user active check
	if auth.GetUserID().Valid() {
		if err = h.isUserActive(ctx, auth); err != nil {
			InactiveUserError(ctx, h, w)
			return
		}
	}

	ctx = platcontext.SetAuthorizer(ctx, auth)

	h.Handler.ServeHTTP(w, r.WithContext(ctx))
}

func (h *AuthenticationHandler) isUserActive(ctx context.Context, auth platform.Authorizer) error {
	u, err := h.UserService.FindUserByID(ctx, auth.GetUserID())
	if err != nil {
		return err
	}

	if u.Status != "inactive" {
		return nil
	}

	return &platform.Error{Code: platform.EForbidden, Msg: "User is inactive"}
}

func (h *AuthenticationHandler) extractAuthorization(ctx context.Context, r *http.Request) (platform.Authorizer, error) {
	t, err := GetToken(r)
	if err != nil {
		return nil, err
	}

	token, err := h.TokenParser.Parse(t)
	if err == nil {
		return token, nil
	}

	// if the error returned signifies ths token is
	// not a well formed JWT then use it as a lookup
	// key for its associated authorization
	// otherwise return the error
	if !jsonweb.IsMalformedError(err) {
		return nil, err
	}

	return h.AuthorizationService.FindAuthorizationByToken(ctx, t)
}

func (h *AuthenticationHandler) extractSession(ctx context.Context, r *http.Request) (*platform.Session, error) {
	k, err := decodeCookieSession(ctx, r)
	if err != nil {
		return nil, err
	}

	s, err := h.SessionService.FindSession(ctx, k)
	if err != nil {
		return nil, err
	}

	if !h.SessionRenewDisabled {
		// if the session is not expired, renew the session
		err = h.SessionService.RenewSession(ctx, s, time.Now().Add(platform.RenewSessionTime))
		if err != nil {
			return nil, err
		}
	}

	return s, err
}
