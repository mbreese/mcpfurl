FROM mcr.microsoft.com/devcontainers/go:2-1.24-bookworm
RUN apt-get update \
    && apt-get install -y chromium-driver sqlite3 vim \
#        gcc-x86-64-linux-gnu gcc-aarch64-linux-gnu \
#    && mkdir -p /opt/musl-cross \
#    && wget https://musl.cc/x86_64-linux-musl-cross.tgz \
#    && wget https://musl.cc/aarch64-linux-musl-cross.tgz \
#    && tar -xzf x86_64-linux-musl-cross.tgz -C /opt/musl-cross \
#    && tar -xzf aarch64-linux-musl-cross.tgz -C /opt/musl-cross
    && apt-get clean 

EXPOSE 8080
#ENV PATH="/opt/musl-cross/x86_64-linux-musl-cross/bin:/opt/musl-cross/aarch64-linux-musl-cross/bin:${PATH}"
