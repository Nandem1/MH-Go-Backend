package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware crea un middleware de logging estructurado similar a Morgan
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Determinar el color del status code
		statusColor := getStatusColor(param.StatusCode)
		methodColor := getMethodColor(param.Method)

		// Calcular duración en milisegundos
		latency := param.Latency.Milliseconds()

		// Formato similar a Morgan
		logLine := fmt.Sprintf(
			"%s %s %s %s %d %s %s %s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			methodColor+param.Method+resetColor,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			statusColor+fmt.Sprintf("%d", param.StatusCode)+resetColor,
			fmt.Sprintf("%dms", latency),
			param.ClientIP,
		)

		// Log estructurado para debugging
		logger.Info("HTTP Request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.String("client_ip", param.ClientIP),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.Int("status_code", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.Time("timestamp", param.TimeStamp),
		)

		return logLine
	})
}

// RequestIDMiddleware agrega un ID único a cada request para tracking
func RequestIDMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	})
}

func getStatusColor(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return greenColor
	case statusCode >= 300 && statusCode < 400:
		return cyanColor
	case statusCode >= 400 && statusCode < 500:
		return yellowColor
	case statusCode >= 500:
		return redColor
	default:
		return whiteColor
	}
}

func getMethodColor(method string) string {
	switch method {
	case "GET":
		return greenColor
	case "POST":
		return blueColor
	case "PUT":
		return yellowColor
	case "DELETE":
		return redColor
	case "PATCH":
		return magentaColor
	default:
		return whiteColor
	}
}

func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
