ARG BASE_IMAGE=opensuse/leap:15.2

FROM golang:1.14.7 AS build
WORKDIR /go/src/github.com/carrier

COPY . .
RUN make

FROM $BASE_IMAGE
LABEL org.opencontainers.image.source https://github.com/SUSE/carrier
RUN zypper ref && zypper install -y git curl tar gzip

# Get kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
  mv kubectl /usr/bin/kubectl && \
  chmod +x /usr/bin/kubectl

# Get helm
RUN curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 && \
  chmod 700 get_helm.sh && \
  ./get_helm.sh && \
  rm get_helm.sh

COPY --from=build /go/src/github.com/carrier/dist/carrier-linux-amd64 /carrier
ENTRYPOINT ["/carrier"]
