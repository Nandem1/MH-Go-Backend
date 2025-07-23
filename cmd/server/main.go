package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stock-service/internal/cache"
	"stock-service/internal/config"
	"stock-service/internal/database"
	"stock-service/internal/handlers"
	"stock-service/internal/middleware"
	"stock-service/internal/repository"
	"stock-service/internal/routes"
	"stock-service/internal/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Configurar logger según ambiente
	logger := configureLogger()
	defer logger.Sync()

	logger.Info("Initializing Stock Service...")

	// Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Configurar modo de Gin
	gin.SetMode(cfg.Server.GinMode)

	// Conectar a PostgreSQL
	postgresDB, err := database.NewPostgresDB(
		cfg.Database.URL,
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer postgresDB.Close()

	// Conectar a Redis
	redisDB, err := database.NewRedisDB(
		cfg.Redis.URL,
		cfg.Redis.Password,
		cfg.Redis.DB,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisDB.Close()

	// Crear ProductCache para POS
	productCache := cache.NewProductCache(
		redisDB.Client,
		1000,           // Max 1000 productos en L1 cache
		30*time.Minute, // TTL de 30 minutos
		logger,
	)

	// Crear repositories
	stockRepo, err := repository.NewStockRepository(postgresDB.DB)
	if err != nil {
		logger.Fatal("Failed to create stock repository", zap.Error(err))
	}

	productRepo, err := repository.NewProductRepository(postgresDB.DB, logger)
	if err != nil {
		logger.Fatal("Failed to create product repository", zap.Error(err))
	}

	// Crear service
	stockService := services.NewStockService(stockRepo, productRepo, redisDB.Client, logger)

	// Crear monitoring service
	monitoringService := services.NewMonitoringService(
		logger,
		cfg,
		redisDB.Client,
		postgresDB.DB,
		productCache,
	)

	// Crear handlers
	stockHandler := handlers.NewStockHandler(stockService, logger)
	posHandler := handlers.NewPOSHandler(productCache, stockService, logger)
	monitoringHandler := handlers.NewMonitoringHandler(monitoringService, logger)

	// Crear health checker
	healthChecker := middleware.NewHealthChecker(postgresDB, redisDB, logger)

	// Configurar router
	router := gin.New()

	// Middleware global
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggerMiddleware(logger))
	router.Use(monitoringHandler.RecordRequestMiddleware()) // Middleware de monitoring

	// Configurar rutas
	routes.SetupRoutes(router, stockHandler, posHandler, monitoringHandler, healthChecker)

	// Configurar servidor
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para señales de terminación
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Mostrar información del servidor
	middleware.ServerInfo(cfg.Server.Port, logger)

	// Iniciar servidor en goroutine
	go func() {
		logger.Info("Starting server", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Esperar señal de terminación
	<-quit
	logger.Info("Shutting down server...")

	// Configurar contexto para shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Cerrar servidor gracefulmente
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// configureLogger configura el logger según el ambiente
func configureLogger() *zap.Logger {
	// Obtener ambiente desde variable de entorno
	env := os.Getenv("ENV")
	if env == "" {
		env = "production" // Default a production
	}

	var logger *zap.Logger
	var err error

	switch env {
	case "development", "debug":
		// Logger de desarrollo con más detalles
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err = config.Build()
		if err != nil {
			panic("Failed to create development logger: " + err.Error())
		}
		logger.Info("🔧 [DEBUG] Logger configurado en modo DESARROLLO - Logs detallados habilitados")

	case "test":
		// Logger para tests
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		logger, err = config.Build()
		if err != nil {
			panic("Failed to create test logger: " + err.Error())
		}

	default: // production
		// Logger de producción - solo logs importantes
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err = config.Build()
		if err != nil {
			panic("Failed to create production logger: " + err.Error())
		}
		logger.Info("🚀 [PROD] Logger configurado en modo PRODUCCIÓN - Solo logs importantes")
	}

	return logger
}
