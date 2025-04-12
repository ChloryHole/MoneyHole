# === Билдер (сборка приложения) ===
FROM golang:1.24.1-alpine AS builder

# Установка зависимостей для сборки (если нужны)
RUN apk add --no-cache git make

# Рабочая директория
WORKDIR /app

# Копируем только модули для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код и собираем
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /moneyhole ./cmd/moneyhole/main.go

# === Финальный образ ===
FROM alpine:3.18

# Безопасность: запуск от непривилегированного пользователя
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Копируем бинарник и TLS-сертификаты (если используются)
COPY --from=builder --chown=appuser:appgroup /moneyhole /app/moneyhole
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/  # раскомментировать если нужны HTTPS-вызовы

# Точка входа
ENTRYPOINT ["/app/moneyhole"]