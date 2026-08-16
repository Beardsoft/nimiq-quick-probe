FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /health-probe ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /health-probe /health-probe
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/health-probe"]
