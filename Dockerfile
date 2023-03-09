FROM golang:latest
RUN mkdir -p /slackbot/config
COPY app /slackbot/
WORKDIR /slackbot
RUN go build -o choowie *.go
CMD ["/slackbot/choowie"]