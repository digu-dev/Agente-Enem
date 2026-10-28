FROM golang:1.27.3-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/retrieval-svc .

FROM scratch
COPY --from=build /out/retrieval-svc /retrieval-svc
USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/retrieval-svc"]
