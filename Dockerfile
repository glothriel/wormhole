FROM golang:1.18-alpine as builder
RUN mkdir /app 
WORKDIR /app 
RUN apk add upx
ADD go.mod go.sum /app/
RUN go mod download -x
ADD main.go /app/
ADD pkg/ /app/pkg
RUN go build -ldflags="-s -w" -o /usr/bin/wormhole main.go
# upx compresses the binary, trading startup time (several hundered ms added) for smaller image size (~30%)
RUN upx /usr/bin/wormhole
RUN chmod +x /usr/bin/wormhole


FROM alpine:latest as runner
RUN apk add tzdata
RUN adduser wormhole --uid 1000 --disabled-password
USER wormhole

FROM runner
COPY --from=builder /usr/bin/wormhole /usr/bin/wormhole

CMD ["/usr/bin/wormhole"]