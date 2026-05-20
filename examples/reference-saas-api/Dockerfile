FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY --from=build /out/migrate /migrate
COPY migrations /migrations
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/api"]
