#editable variables
ARG GO_VERSION=1.13
ARG DEFAULT_APP_NAME="agt_server"
ARG DEFAULT_APP_PATH="/app"
ARG DEFAULT_SRCDIR="server/${DEFAULT_APP_NAME}/"

#dont edit below this line
FROM golang:${GO_VERSION}-alpine AS dev
ARG DEFAULT_APP_NAME
ARG DEFAULT_APP_PATH
ARG DEFAULT_SRCDIR

ENV APP_NAME=${DEFAULT_APP_NAME} \
    APP_PATH=${DEFAULT_APP_PATH}

WORKDIR ${APP_PATH}
COPY ${DEFAULT_SRCDIR} .
COPY go.mod go.sum ./

#compile agtserver
RUN go build -o ${APP_NAME} .

FROM alpine AS agt-server
ARG DEFAULT_APP_NAME
ARG DEFAULT_APP_PATH

ENV APP_NAME=${DEFAULT_APP_NAME} \
    APP_PATH=${DEFAULT_APP_PATH}

WORKDIR ${APP_PATH}
COPY --from=dev ${APP_PATH}/${APP_NAME} .

ENTRYPOINT ./${APP_NAME}
CMD ""
