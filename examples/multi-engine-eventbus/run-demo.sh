#!/bin/bash

# Multi-Engine EventBus Demo Setup Script
# This script helps set up and run the multi-engine eventbus example

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker and Docker Compose are available
check_dependencies() {
    print_status "Checking dependencies..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed or not in PATH"
        exit 1
    fi
    
    print_success "Dependencies check passed"
}

# Start the services
start_services() {
    print_status "Starting Redis and Kafka services..."
    
    # Use docker compose (newer) or docker-compose (older)
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    $COMPOSE_CMD up -d
    
    print_status "Waiting for services to be ready..."
    
    # Wait for Redis
    print_status "Waiting for Redis to be ready..."
    for i in {1..30}; do
        if docker exec eventbus-redis redis-cli ping | grep -q PONG; then
            print_success "Redis is ready"
            break
        fi
        if [ $i -eq 30 ]; then
            print_error "Redis failed to start after 30 attempts"
            exit 1
        fi
        sleep 1
    done
    
    # Wait for Kafka
    print_status "Waiting for Kafka to be ready..."
    for i in {1..60}; do
        if docker exec eventbus-kafka kafka-topics --bootstrap-server localhost:9092 --list &> /dev/null; then
            print_success "Kafka is ready"
            break
        fi
        if [ $i -eq 60 ]; then
            print_error "Kafka failed to start after 60 attempts"
            exit 1
        fi
        sleep 1
    done
    
    print_success "All services are ready!"
}

# Stop the services
stop_services() {
    print_status "Stopping services..."
    
    # Use docker compose (newer) or docker-compose (older)
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    $COMPOSE_CMD down
    print_success "Services stopped"
}

# Clean up (remove volumes too)
cleanup_services() {
    print_status "Cleaning up services and volumes..."
    
    # Use docker compose (newer) or docker-compose (older)
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    $COMPOSE_CMD down -v
    print_success "Services and volumes removed"
}

# Run the application
run_app() {
    print_status "Building and running the multi-engine eventbus example..."
    go run main.go
}

# Show usage
usage() {
    echo "Multi-Engine EventBus Demo Setup Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  start    - Start Redis and Kafka services"
    echo "  stop     - Stop the services"
    echo "  cleanup  - Stop services and remove volumes"
    echo "  run      - Start services and run the Go application"
    echo "  app      - Run only the Go application (services must be running)"
    echo "  status   - Show the status of running services"
    echo "  logs     - Show logs from all services"
    echo ""
    echo "Examples:"
    echo "  $0 run        # Start everything and run the demo"
    echo "  $0 start      # Just start the services"
    echo "  $0 app        # Run the app (services must be running)"
    echo "  $0 cleanup    # Clean up everything"
}

# Show status
show_status() {
    print_status "Service status:"
    echo ""
    
    # Use docker compose (newer) or docker-compose (older)
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    $COMPOSE_CMD ps
}

# Show logs
show_logs() {
    print_status "Service logs:"
    
    # Use docker compose (newer) or docker-compose (older)
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    
    $COMPOSE_CMD logs -f --tail=100
}

# Main script logic
case "${1:-run}" in
    "start")
        check_dependencies
        start_services
        ;;
    "stop")
        stop_services
        ;;
    "cleanup")
        cleanup_services
        ;;
    "run")
        check_dependencies
        start_services
        echo ""
        print_success "Services are ready! Starting the application..."
        echo ""
        run_app
        ;;
    "app")
        run_app
        ;;
    "status")
        show_status
        ;;
    "logs")
        show_logs
        ;;
    "help"|"-h"|"--help")
        usage
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        usage
        exit 1
        ;;
esac