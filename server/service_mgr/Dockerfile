#editable variables
ARG GO_VERSION=1.13
ARG DEFAULT_APP_NAME="service_mgr"
ARG DEFAULT_APP_PATH="/app"
ARG DEFAULT_SRCDIR="server/${DEFAULT_APP_NAME}/"

#dont edit below this line
FROM golang:${GO_VERSION}-alpine AS dev
ARG DEFAULT_APP_NAME
ARG DEFAULT_APP_PATH
ARG DEFAULT_SRCDIR
RUN apk add --no-cache gcc
RUN apk add --no-cache musl-dev

ENV APP_NAME=${DEFAULT_APP_NAME} \
    APP_PATH=${DEFAULT_APP_PATH} 

WORKDIR ${APP_PATH}
COPY ${DEFAULT_SRCDIR} .
COPY go.mod go.sum ./

#compile servercore
RUN go build -o ${APP_NAME} .

FROM alpine AS servermgr
ARG DEFAULT_APP_NAME
ARG DEFAULT_APP_PATH

ENV APP_NAME=${DEFAULT_APP_NAME} \
    APP_PATH=${DEFAULT_APP_PATH}

WORKDIR ${APP_PATH}
COPY --from=dev ${APP_PATH}/${APP_NAME} .
EXPOSE 8080

ENTRYPOINT ./${APP_NAME}
CMD ""
