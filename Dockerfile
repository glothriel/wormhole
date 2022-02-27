FROM golang:alpine as builder
RUN mkdir /app 
WORKDIR /app 
ADD go.mod /app/
RUN go mod download -x
ADD . /app/
RUN go build -o /usr/bin/wormhole main.go
RUN chmod +x /usr/bin/wormhole


FROM alpine:3.15.0 as runner
RUN apk add tzdata
RUN adduser wormhole --uid 1000 --disabled-password
USER wormhole

FROM runner
COPY --from=builder /usr/bin/wormhole /usr/bin/wormhole

CMD ["/usr/bin/wormhole"]