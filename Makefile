# Variables
BINARY_NAME=stock-service
BUILD_DIR=bin
MAIN_FILE=cmd/server/main.go

# Colores para output
GREEN=\033[0;32m
NC=\033[0m # No Color
YELLOW=\033[1;33m

.PHONY: help build run test clean dev docker-build docker-run

# Comando por defecto
help: ## Mostrar esta ayuda
	@echo "$(GREEN)Stock Service - Comandos disponibles:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'

build: ## Compilar el proyecto
	@echo "$(GREEN)Compilando...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)Compilado exitosamente en $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

run: ## Ejecutar el servidor
	@echo "$(GREEN)Ejecutando servidor...$(NC)"
	go run $(MAIN_FILE)

dev: ## Ejecutar en modo desarrollo
	@echo "$(GREEN)Ejecutando en modo desarrollo...$(NC)"
	GIN_MODE=development go run $(MAIN_FILE)

dev-hot: ## Ejecutar con hot reload (requiere air instalado)
	@echo "$(GREEN)Ejecutando con hot reload...$(NC)"
	@if ! command -v air &> /dev/null; then \
		echo "$(YELLOW)Instalando Air...$(NC)"; \
		go install github.com/cosmtrek/air@latest; \
	fi
	GIN_MODE=development air

install-air: ## Instalar Air para hot reload
	@echo "$(GREEN)Instalando Air...$(NC)"
	go install github.com/cosmtrek/air@latest

test: ## Ejecutar tests
	@echo "$(GREEN)Ejecutando tests...$(NC)"
	go test -v ./...

test-coverage: ## Ejecutar tests con coverage
	@echo "$(GREEN)Ejecutando tests con coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generado en coverage.html$(NC)"

clean: ## Limpiar archivos generados
	@echo "$(GREEN)Limpiando...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean

deps: ## Descargar dependencias
	@echo "$(GREEN)Descargando dependencias...$(NC)"
	go mod download
	go mod tidy

fmt: ## Formatear código
	@echo "$(GREEN)Formateando código...$(NC)"
	go fmt ./...

lint: ## Ejecutar linter
	@echo "$(GREEN)Ejecutando linter...$(NC)"
	golangci-lint run

docker-build: ## Construir imagen Docker
	@echo "$(GREEN)Construyendo imagen Docker...$(NC)"
	docker build -t $(BINARY_NAME) .

docker-run: ## Ejecutar contenedor Docker
	@echo "$(GREEN)Ejecutando contenedor Docker...$(NC)"
	docker run -p 8080:8080 --env-file .env $(BINARY_NAME)

install-tools: ## Instalar herramientas de desarrollo
	@echo "$(GREEN)Instalando herramientas...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-delve/delve/cmd/dlv@latest

# Comando para verificar que todo esté listo
check: ## Verificar que todo esté configurado correctamente
	@echo "$(GREEN)Verificando configuración...$(NC)"
	@go version
	@echo "$(GREEN)Go version OK$(NC)"
	@go mod verify
	@echo "$(GREEN)Go modules OK$(NC)"
	@go build -o /dev/null $(MAIN_FILE)
	@echo "$(GREEN)Build OK$(NC)"
	@echo "$(GREEN)✅ Todo listo!$(NC)" 