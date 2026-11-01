FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mealplan ./cmd/mealplan
RUN mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mealplan /mealplan
COPY --from=build --chown=65532:65532 /out/data /data
ENV MEALPLAN_ADDR=:8080 \
    MEALPLAN_DB_DSN="file:/data/mealplan.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/mealplan"]
