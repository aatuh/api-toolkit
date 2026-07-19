FROM golang:1.25@sha256:d2e20dc1b35aefd666909163e4ace41efb521359aa2ce31fff59d86837050f6f AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b
WORKDIR /
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY --from=build /out/migrate /migrate
COPY migrations /migrations
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/api"]
