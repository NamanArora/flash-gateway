# Multi-stage build for production-ready dashboard
# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm ci

# Copy source code and build
COPY . .
RUN npm run build

# Stage 2: Production image
FROM node:20-alpine
WORKDIR /app

# Install netcat for health checks
RUN apk add --no-cache netcat-openbsd

# Copy backend server files
COPY server ./server
COPY package*.json ./

# Install all dependencies for development mode
RUN npm ci

# Copy built frontend from stage 1
COPY --from=frontend-builder /app/dist ./dist

# Copy entrypoint script
COPY docker-entrypoint.sh ./
RUN chmod +x docker-entrypoint.sh

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \
    adduser -S dashboard -u 1001 -G nodejs

# Change ownership of app directory
RUN chown -R dashboard:nodejs /app
USER dashboard

# Expose ports
EXPOSE 4000 5173

# Start using entrypoint script
ENTRYPOINT ["./docker-entrypoint.sh"]