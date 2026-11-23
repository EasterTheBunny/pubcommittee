# Development image: every build, test, and lint runs in it, for agents and CI.
# Go toolchain at the version pinned in go.mod, Node at the version app/ builds with.
# Go tools (golangci-lint, oapi-codegen) come from the module's tool directive:
# run them with `go tool <name>`.
FROM node:24-bookworm-slim@sha256:dc26ae92c9fb15cf6b1d63e20f0944b2efe0f115b20e00be48fb23fceb061c54 AS node

FROM golang:1.27@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
 && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
ENV GOFLAGS=-buildvcs=false
WORKDIR /src
CMD ["sleep", "infinity"]
