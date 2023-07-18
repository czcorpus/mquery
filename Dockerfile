FROM czcorpus/kontext-manatee:2.223.6-jammy

RUN apt-get update && apt-get install wget tar python3-dev python3-pip curl git bison -y \
    && wget https://go.dev/dl/go1.20.6.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.20.6.linux-amd64.tar.gz \
    && pip install pulp numpy

COPY . /opt/mquery
WORKDIR /opt/mquery

RUN git config --global --add safe.directory /opt/mquery \
    && export PATH=$PATH:/usr/local/go/bin \
    && python3 build3 2.223.6

EXPOSE 8088
CMD ["./mquery", "start", "conf-docker.json"]