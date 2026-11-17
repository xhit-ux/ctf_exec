FROM alpine:3.22 AS kata
COPY .local/kata-static-4.3.1-amd64.tar.zst /kata.tar.zst
RUN echo '1463a2b86845f4d3d8e87f2f764ec59856fc0fee3520967a79ade32762be2e97  /kata.tar.zst' | sha256sum -c - && \
    apk add --no-cache zstd tar && tar -I zstd -xf /kata.tar.zst -C /

FROM rancher/k3s:v1.34.13-k3s1
COPY --from=kata /opt/kata /opt/kata
COPY config-v3.toml.tmpl /var/lib/rancher/k3s/agent/etc/containerd/config-v3.toml.tmpl
