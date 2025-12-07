package api

import (
	"net/http"
	"time"

	"trading-core/internal/balance"
	"trading-core/internal/events"
	"trading-core/internal/monitor"
	"trading-core/internal/order"
	"trading-core/internal/risk"
	"trading-core/internal/strategy"
	"trading-core/pkg/db"

	"github.com/gin-gonic/gin"
)

// Server wires HTTP endpoints around the event bus.
type Server struct {
	Router     *gin.Engine
	Bus        *events.Bus
	DB         *db.Database
	RiskMgr    *risk.Manager
	BalanceMgr *balance.Manager
	StratEng   *strategy.Engine
	Metrics    *monitor.SystemMetrics
	OrderQueue *order.Queue
	JWTSecret  string
	Meta       SystemMeta
}

// SystemMeta describes runtime status exposed to the UI.
type SystemMeta struct {
	DryRun      bool
	Venue       string
	Symbols     []string
	UseMockFeed bool
	Version     string
}

func NewServer(bus *events.Bus, database *db.Database, riskMgr *risk.Manager, balanceMgr *balance.Manager, stratEng *strategy.Engine, metrics *monitor.SystemMetrics, orderQueue *order.Queue, meta SystemMeta, jwtSecret string) *Server {
	r := gin.New()

	// Middleware stack (order matters!)
	r.Use(gin.Recovery())        // Panic recovery (first)
	r.Use(RequestIDMiddleware()) // Request ID tracking
	r.Use(RequestLogger())       // Request logging (after ID is set)
	r.Use(RateLimitMiddleware()) // Rate limiting
	// Security headers handled by Nginx
	r.Use(TimeoutMiddleware(30 * time.Second)) // Request timeout (30s)
	r.Use(CORSMiddleware())                    // CORS (last before routes)

	s := &Server{
		Router:     r,
		Bus:        bus,
		DB:         database,
		RiskMgr:    riskMgr,
		BalanceMgr: balanceMgr,
		StratEng:   stratEng,
		Metrics:    metrics,
		OrderQueue: orderQueue,
		JWTSecret:  jwtSecret,
		Meta:       meta,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.GET("/health", s.health)
	s.Router.GET("/ws", s.websocket)

	api := s.Router.Group("/api")
	{
		api.GET("/system/status", s.getSystemStatus)
		api.GET("/metrics", s.getMetrics)
		api.GET("/queue/metrics", s.getQueueMetrics)

		// Auth endpoints (no auth required)
		auth := api.Group("/auth")
		{
			auth.POST("/register", s.registerUser)
			auth.POST("/login", s.loginUser)
		}

		// Protected API
		protected := api.Group("")
		protected.Use(AuthMiddleware(s.JWTSecret))
		{
			protected.GET("/strategies", s.getStrategies)
			protected.GET("/orders", s.getOrders)
			protected.GET("/positions", s.getPositions)
			protected.GET("/balance", s.getBalance)
			protected.GET("/risk", s.getRiskMetrics)
			protected.GET("/strategies/:id/performance", s.getStrategyPerformance)

			// Strategy Actions
			protected.POST("/strategies/:id/start", s.startStrategy)
			protected.POST("/strategies/:id/pause", s.pauseStrategy)
			protected.POST("/strategies/:id/stop", s.stopStrategy)
			protected.POST("/strategies/:id/panic", s.panicSellStrategy)
			protected.PUT("/strategies/:id/params", s.updateStrategyParams)
			protected.PUT("/strategies/:id/binding", s.updateStrategyBinding)

			// Exchange connections (Phase 2)
			protected.GET("/connections", s.listConnections)
			protected.POST("/connections", s.createConnection)
			protected.DELETE("/connections/:id", s.deactivateConnection)
		}
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) Start(addr string) error {
	return s.Router.Run(addr)
}
