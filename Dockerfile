FROM maven:3.6-jdk-7 as builder-mvn
RUN git clone https://github.com/newrelic/nrjmx.git && \
    cd nrjmx && \
    mvn clean package -P \!deb,\!rpm

FROM golang:1.10 as builder
RUN go get -d github.com/newrelic/nri-jmx/... && \
    cd /go/src/github.com/newrelic/nri-jmx && \
    make && \
    strip ./bin/nr-jmx

FROM newrelic/infrastructure:latest
ENV NRIA_IS_FORWARD_ONLY true
ENV NRIA_K8S_INTEGRATION true
RUN apk --update add openjdk7-jre
COPY --from=builder-mvn /nrjmx/bin/nrjmx /usr/bin/nrjmx
COPY --from=builder-mvn /nrjmx/bin/nrjmx.jar /usr/bin/nrjmx.jar
COPY --from=builder /go/src/github.com/newrelic/nri-jmx/bin/nr-jmx /var/db/newrelic-infra/newrelic-integrations/bin/nr-jmx
COPY --from=builder /go/src/github.com/newrelic/nri-jmx/jmx-definition.yml /var/db/newrelic-infra/newrelic-integrations/definition.yaml
