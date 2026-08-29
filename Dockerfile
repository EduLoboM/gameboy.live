FROM golang:1.27-bookworm AS builder

RUN apt-get update && apt-get install -y \
    libasound2-dev \
    libgl1-mesa-dev \
    xorg-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gbdotlive main.go

FROM debian:bookworm-slim
# Adicionamos o wget aqui para podermos baixar a ROM
RUN apt-get update && apt-get install -y \
    wget \
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
COPY gb.svg .

RUN mkdir -p snapshots /data 

EXPOSE 8000

# Se a variável ROM_URL estiver configurada, baixa o arquivo como game.gbc.
# Em seguida, inicia o servidor apontando para game.gbc
CMD ["/bin/sh", "-c", "if [ ! -z \"$ROM_URL\" ]; then wget -qO game.gbc \"$ROM_URL\"; else echo 'Nenhuma ROM_URL fornecida!'; exit 1; fi; ./gbdotlive -S -p 8000 -r game.gbc"]