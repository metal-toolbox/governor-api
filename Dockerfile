FROM gcr.io/distroless/static:nonroot

# `nonroot` coming from distroless
USER 65532:65532

COPY governor-api /governor-api

# Run the web service on container startup.
ENTRYPOINT ["/governor-api"]
CMD ["serve"]
