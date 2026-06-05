# deploy/Dockerfile — build the devops-toolkit image used by docker-compose.
# Multi-stage: builder produces a static binary; runtime is distroless.
FROM golang:1.25 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/devops-toolkit ./cmd/devops-toolkit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/devops-toolkit /usr/local/bin/devops-toolkit
EXPOSE 3000
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/devops-toolkit"]
