FROM golang:1.8

WORKDIR /go/src/app
COPY . .


RUN go get -d -v ./...
RUN go install -v ./...


CMD ["app", "-host", "0.0.0.0", "-port", "80"]