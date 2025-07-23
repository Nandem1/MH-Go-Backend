package middleware

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"go.uber.org/zap"
)

// ServerInfo muestra información profesional del servidor al iniciar
func ServerInfo(port string, logger *zap.Logger) {
	// Información del sistema
	hostname, _ := os.Hostname()

	// Información de Go
	goVersion := runtime.Version()
	numCPU := runtime.NumCPU()

	// Tiempo de inicio
	startTime := time.Now().Format("2006-01-02 15:04:05")

	// Banner del servidor
	fmt.Println("")
	fmt.Println("🚀 " + boldColor + "Stock Service API" + resetColor)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("📅 Started at: " + startTime)
	fmt.Println("🌐 Server URL: " + cyanColor + "http://localhost:" + port + resetColor)
	fmt.Println("💻 Hostname: " + hostname)
	fmt.Println("🔧 Go Version: " + goVersion)
	fmt.Println("⚡ CPU Cores: " + fmt.Sprintf("%d", numCPU))
	fmt.Println("")
	fmt.Println("📊 " + boldColor + "Available Endpoints:" + resetColor)
	fmt.Println("   GET  " + greenColor + "/" + resetColor + "          - API Information")
	fmt.Println("   GET  " + greenColor + "/health" + resetColor + "       - Health Check")
	fmt.Println("")
	fmt.Println("🔍 " + boldColor + "Monitoring:" + resetColor)
	fmt.Println("   📈 Health Check: " + cyanColor + "http://localhost:" + port + "/health" + resetColor)
	fmt.Println("")
	fmt.Println("⚙️  " + boldColor + "Environment:" + resetColor)
	fmt.Println("   🗄️  Database: PostgreSQL (Railway)")
	fmt.Println("   🗃️  Cache: Redis (Railway)")
	fmt.Println("   📝 Logging: Structured (Zap)")
	fmt.Println("")
	fmt.Println("🎯 " + boldColor + "Next Steps:" + resetColor)
	fmt.Println("   📦 Step 2: Models & Repository Layer")
	fmt.Println("   🔧 Step 3: Service Layer")
	fmt.Println("   🌐 Step 4: HTTP Handlers")
	fmt.Println("   ⚡ Step 5: Optimizations")
	fmt.Println("")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✨ " + boldColor + "Server is ready to handle requests!" + resetColor)
	fmt.Println("")

	// Log estructurado
	logger.Info("Server started successfully",
		zap.String("port", port),
		zap.String("hostname", hostname),
		zap.String("go_version", goVersion),
		zap.Int("cpu_cores", numCPU),
		zap.String("start_time", startTime),
	)
}
