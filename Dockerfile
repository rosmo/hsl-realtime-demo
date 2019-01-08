FROM golang:latest 
RUN mkdir -p /app/src/github.com/rosmo/hsl-realtime-demo
ADD reader.go /app/src/github.com/rosmo/hsl-realtime-demo
WORKDIR /app/src/github.com/rosmo/hsl-realtime-demo
ENV GOPATH /app
RUN go get
RUN go build -o hsl-realtime-demo . 


ENTRYPOINT [ "/app/src/github.com/rosmo/hsl-realtime-demo/hsl-realtime-demo" ]
CMD [ "-geohash=5" ]
