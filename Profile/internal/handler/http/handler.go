package http

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/handler/middleware"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/websocket"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	"github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorilla_ws "github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	userCtx = "userID"
)

type Handler struct {
	usersService service.UsersService
	coinsService service.CoinsService
	log          *slog.Logger
	jwtSecret    string
	wsManager    *websocket.Manager
	upgrader     gorilla_ws.Upgrader
	httpClient   *http.Client
	authClient   auth.AuthClient
}

func NewHandler(usersService service.UsersService, coinsService service.CoinsService, wsManager *websocket.Manager, log *slog.Logger, jwtSecret string, authClient auth.AuthClient) *Handler {
	return &Handler{
		usersService: usersService,
		coinsService: coinsService,
		wsManager:    wsManager,
		log:          log,
		jwtSecret:    jwtSecret,
		upgrader: gorilla_ws.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		httpClient: &http.Client{},
		authClient: authClient,
	}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", h.register)
			auth.POST("/login", h.login)
		}

		profile := api.Group("/profile", middleware.AuthMiddleware(h.jwtSecret, h.log))
		{
			profile.GET("", h.getUserProfile)
			profile.POST("", h.createUserProfile)
			profile.POST("/coins", h.updateCoinQuantity)
			profile.DELETE("/coins", h.deleteCoin)
		}
		ws := api.Group("/ws", middleware.AuthMiddleware(h.jwtSecret, h.log))
		{
			ws.GET("", h.wsConnect)
		}
	}
}

type authRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	grpcResp, err := h.authClient.Register(c.Request.Context(), &auth.RegisterRequest{
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		st, _ := status.FromError(err)
		c.JSON(grpcCodeToHTTP(st.Code()), gin.H{"error": st.Message()})
		return
	}

	userID, err := uuid.Parse(grpcResp.GetUserId())
	if err != nil {
		h.log.Error("failed to parse userID from grpc response", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if _, err := h.usersService.CreateUserProfile(c.Request.Context(), userID, req.Name); err != nil {
		if !errors.Is(err, errs.ErrAlreadyExists) {
			h.log.Error("failed to create user profile after registration", "error", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}

func (h *Handler) login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	grpcResp, err := h.authClient.Login(c.Request.Context(), &auth.LoginRequest{
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		st, _ := status.FromError(err)
		c.JSON(grpcCodeToHTTP(st.Code()), gin.H{"error": st.Message()})
		return
	}

	c.SetCookie("refreshToken", grpcResp.GetRefreshToken(), int(time.Hour*24*30/time.Second), "/", "localhost", false, true)

	c.JSON(http.StatusOK, gin.H{"accessToken": grpcResp.GetAccessToken()})
}

func (h *Handler) wsConnect(c *gin.Context) {
	userIDRaw, _ := c.Get(userCtx)
	userID, _ := uuid.Parse(userIDRaw.(string))

	userProfile, err := h.usersService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			userName, _ := c.Get("userName")
			userProfile, err = h.usersService.CreateUserProfile(c.Request.Context(), userID, userName.(string))
			if err != nil {
				h.log.Error("ws: failed to auto-create user profile", "error", err, "userID", userID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not authorize websocket"})
				return
			}
		} else {
			h.log.Error("ws: cannot get user profile", "error", err, "userID", userID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not authorize websocket"})
			return
		}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("failed to upgrade connection", "error", err)
		return
	}

	client := &websocket.Client{
		Manager: h.wsManager,
		Conn:    conn,
		UserID:  userID,
		Profile: userProfile,
		Send:    make(chan []byte, 256),
	}

	client.Manager.Register(client)

	go client.Writer()
	go client.Reader()
}

func (h *Handler) getUserProfile(c *gin.Context) {
	userIDRaw, ok := c.Get(userCtx)
	if !ok {
		h.log.Error("handler: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDRaw.(string))
	if err != nil {
		h.log.Error("handler: failed to parse userID from context", "userID", userIDRaw)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
		return
	}

	user, err := h.usersService.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user profile not found"})
			return
		}
		h.log.Error("failed to get user profile", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

type coinRequest struct {
	Symbol   string `json:"symbol" binding:"required"`
	Quantity string `json:"quantity"`
}

func (h *Handler) updateCoinQuantity(c *gin.Context) {
	var req coinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if req.Quantity == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity is required"})
		return
	}

	userIDRaw, _ := c.Get(userCtx)
	userID, _ := uuid.Parse(userIDRaw.(string))

	quantityChange, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quantity format"})
		return
	}

	updatedCoin, err := h.coinsService.UpdateCoinQuantity(c.Request.Context(), userID, req.Symbol, quantityChange)
	if err != nil {
		if errors.Is(err, errs.ErrInsufficientFunds) {
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient funds"})
			return
		}
		h.log.Error("failed to update coin quantity", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update coin"})
		return
	}

	c.JSON(http.StatusOK, updatedCoin)
}

func (h *Handler) deleteCoin(c *gin.Context) {
	var req coinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body, 'symbol' is required"})
		return
	}

	userIDRaw, _ := c.Get(userCtx)
	userID, _ := uuid.Parse(userIDRaw.(string))

	if err := h.coinsService.DeleteCoin(c.Request.Context(), userID, req.Symbol); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "coin not found in portfolio"})
			return
		}
		h.log.Error("failed to delete coin", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete coin"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "coin successfully deleted"})
}

func (h *Handler) createUserProfile(c *gin.Context) {
	userIDRaw, _ := c.Get(userCtx)
	userNameRaw, ok := c.Get("userName")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user name not found in token"})
		return
	}

	userID, _ := uuid.Parse(userIDRaw.(string))
	userName := userNameRaw.(string)

	if _, err := h.usersService.CreateUserProfile(c.Request.Context(), userID, userName); err != nil {
		if errors.Is(err, errs.ErrAlreadyExists) {
			c.JSON(http.StatusOK, gin.H{"message": "profile already exists"})
			return
		}
		h.log.Error("failed to create user profile", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create profile"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "profile created successfully"})
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Internal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
