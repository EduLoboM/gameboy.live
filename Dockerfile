FROM golang:1.21-bullseye AS builder

RUN apt-get update && apt-get install -y \
    libasound2-dev \
    libgl1-mesa-dev \
    xorg-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o gbdotlive main.go

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y \
    libasound2 \
    libgl1 \
    libxrandr2 \
    libxcursor1 \
    libxinerama1 \
    libxi6 \
    libxxf86vm1 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/gbdotlive .
COPY ["pkmc (patched).gbc", "."]
COPY gb.svg .

RUN mkdir snapshots 

EXPOSE 8000

CMD ["./gbdotlive", "-S", "-p", "8000", "-r", "pkmc (patched).gbc"]