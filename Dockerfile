FROM golang:1.22.7 as builder
ARG MODULE=""
WORKDIR /opt/app
ADD . /opt/app/
ENV CGO_ENABLED=0
RUN mkdir dist && \
  cd ${MODULE} && \
  go build -o ../dist/ . 

FROM alpine:3 as packager
COPY --from=builder /opt/app/dist/${MODULE} /opt/app/
WORKDIR /opt/app/
ENV TZ Asia/Shanghai
