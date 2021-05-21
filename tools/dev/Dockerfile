FROM golang:latest

# golang latest is based on BUSTER - 317M ( vs 108M for alpine )

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOPROXY=https://proxy.golang.org

RUN apt-get update && \
    apt install -y --no-install-recommends \
       less net-tools openssh-server sudo \
       lsof netcat tcpdump iptables iproute2 git && \
    apt-get clean && \
    rm -rf  /var/log/*log /var/lib/apt/lists/* /var/log/apt/* /var/lib/dpkg/*-old /var/cache/debconf/*-old

RUN useradd -u 1000 -U -d /work -s /bin/bash build && \
     usermod -G users build && \
      echo "build ALL=NOPASSWD: ALL" >> /etc/sudoers

RUN useradd -u 1337 -U -d /var/run/istio -s /bin/bash istio-proxy && \
     usermod -G users istio-proxy && \
      echo "istio-proxy ALL=NOPASSWD: ALL" >> /etc/sudoers

#RUN curl -fsSL https://code-server.dev/install.sh > /tmp/install.sh && \
#    sh /tmp/install.sh


RUN mkdir -p /etc/certs && \
    mkdir -p /run/ssh && \
    mkdir -p /etc/istio/proxy && \
    mkdir -p /etc/istio/config && \
    mkdir -p /var/lib/istio/envoy && \
    mkdir -p /var/lib/istio/config && \
    mkdir -p /var/lib/istio/proxy && \
    chown -R 1337 /etc/certs /etc/istio /var/lib/istio && \
    mkdir /run/ugate && \
    ln -s /ko-app/ugate /usr/local/bin/ugate && \
    mkdir /run/sshd && \
    chmod 700 /run/sshd

# Binaries to /go/bin
#RUN go get github.com/githubnemo/CompileDaemon

RUN go get github.com/google/ko@latest && \
    go get github.com/go-delve/delve/cmd/dlv && \
    rm -rf /root/.cache && rm -rf /go/pkg && go clean -cache

# Default is /go directory, which is set a GOPATH
# That doesn't work with go.mod
WORKDIR /ws

EXPOSE 8080
EXPOSE 15007

COPY tools/dev/sshd_config /etc/ssh/sshd_config

# Test only
RUN echo 'root:test' | chpasswd

RUN sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd

COPY tools/dev/run.sh /usr/local/bin/

# Runs in /go directory
#COPY . .
#RUN cd cmd/ugate && go build -a -gcflags='all=-N -l' -ldflags '-extldflags "-static"' \
#  -o /usr/local/bin/ugate ./

# Priv separataion dir

ENTRYPOINT /usr/local/bin/run.sh
