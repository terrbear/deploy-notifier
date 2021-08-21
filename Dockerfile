FROM alpine:latest as build

RUN apk add --no-cache git make musl-dev go 

# Configure Go
ENV GOROOT /usr/lib/go
ENV GOPATH /go
ENV PATH /go/bin:$PATH

RUN mkdir -p ${GOPATH}/src /app

COPY . ${GOPATH}/src

WORKDIR $GOPATH/src

RUN go build -o bin/server main.go \
      && mv bin/* /app

WORKDIR /app

RUN rm -rf $GOPATH/src

FROM alpine:latest

COPY --from=build /app/server /server

WORKDIR /

EXPOSE 8085

CMD "./server"