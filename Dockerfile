FROM scratch
COPY ipmiserial /ipmiserial
COPY config.yaml.example /config.yaml
EXPOSE 8080
ENTRYPOINT ["/ipmiserial"]
CMD ["-config", "/config.yaml"]
