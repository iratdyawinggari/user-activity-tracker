# User Activity Tracker - Complete Setup Guide

## Prerequisites
- Docker and Docker Compose
- Go 1.21 or later
- MySQL Client (optional)
- Redis CLI (optional)

## Quick Start

### Option 1: Using Docker (Recommended)
```bash
# 1. Clone the repository
git clone <repository-url>
cd user-activity-tracker

# 2. Copy environment variables
cp .env.example .env

# 3. Start all services
docker-compose up -d

# 4. Check services are running
docker-compose ps

# 5. Access the application
# API: http://localhost:8080
# Swagger Docs: http://localhost:8080/swagger/index.html
# Health Check: http://localhost:8080/health