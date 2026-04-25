# ---- Сборка приложения ----
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Копируем файлы с зависимостями
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем статический бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./build/tg_bot ./cmd/tg/

# ---- Финальный образ ----
FROM alpine:latest

# Устанавливаем ca-certificates и tzdata для корректной работы HTTPS и временных зон
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем скомпилированный бинарник из предыдущего этапа
COPY --from=builder /app/build/tg_bot .

# Порт, который слушает приложение (измените при необходимости)
EXPOSE 8080

# Запускаем
CMD ["./tg_bot"]
