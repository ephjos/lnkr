FROM golang:alpine

WORKDIR /build

COPY . .

RUN \
	mkdir -p bolt && \
	go build ./cmd/main.go

EXPOSE 3333

CMD ["/build/main"]

