FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache poppler-utils tesseract-ocr tesseract-ocr-data-eng ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
ENV ADDR=:8080 DATA_DIR=/app/data
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/server"]
