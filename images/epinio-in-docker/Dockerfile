# Download tools
FROM busybox:latest AS downloader

RUN wget https://get.helm.sh/helm-v3.9.0-linux-amd64.tar.gz -O helm.tar.gz
RUN mkdir helm
RUN tar xvf helm.tar.gz -C helm
RUN mv helm/*/helm /bin/helm

## Build the actual container
FROM rancher/k3d:5.3

COPY --from=downloader /bin/helm /usr/local/bin/helm

ADD startup.sh /startup.sh

ENTRYPOINT ["/startup.sh"]
