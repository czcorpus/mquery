FROM czcorpus/kontext-manatee:2.223.6-jammy

RUN apt-get update && apt-get install wget tar python3-dev python3-pip curl git bison libpcre3-dev -y \
    && wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz \
    && pip install pulp numpy

WORKDIR /opt
RUN git clone https://github.com/czcorpus/manabuild \
    && cd manabuild \
    && export PATH=$PATH:/usr/local/go/bin \
    && make build && make install

COPY . /opt/mquery
WORKDIR /opt/mquery

RUN git config --global --add safe.directory /opt/mquery \
    && PATH=$PATH:/usr/local/go/bin:/root/go/bin \
    && ./configure \
    && make build

EXPOSE 8088
CMD ["./mquery", "server", "conf-docker.json"]