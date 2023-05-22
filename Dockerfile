FROM golang:1.19.9-alpine as builder
WORKDIR /src
COPY . . 
RUN cd cmd/apmsoak \
  && go mod download \
  && go build

FROM alpine:3.14.2
WORKDIR /src
COPY --from=builder /src/cmd/apmsoak .
COPY ./loadgen/events ./events
ENTRYPOINT ["/src/apmsoak"]