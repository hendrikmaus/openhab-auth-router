FROM scratch
COPY openhab-auth-router /
ENTRYPOINT ["/openhab-auth-router"]
