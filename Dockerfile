FROM golang:1.8.1 AS build
WORKDIR /go/src/github.com/jritsema/k8s-crd-poc
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -o app .

FROM alpine:3.6
RUN apk --no-cache add ca-certificates
COPY --from=build /go/src/github.com/jritsema/k8s-crd-poc/app .
CMD [ "./app" ]  

