FROM gcr.io/distroless/static:nonroot

ARG BIN=governor-api

# `nonroot` coming from distroless
USER 65532:65532

COPY ${BIN} /governor-api

# Run the web service on container startup.
ENTRYPOINT ["/governor-api"]
CMD ["serve"]
