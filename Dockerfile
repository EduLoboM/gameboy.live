# Estágio de Build
FROM golang:1.21-bullseye AS builder

# Instala as dependências de sistema necessárias para compilar o projeto
# conforme exigido pelo README (libasound2-dev e libgl1-mesa-dev)
RUN apt-get update && apt-get install -y \
    libasound2-dev \
    libgl1-mesa-dev \
    xorg-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copia os arquivos de dependência do Go
COPY go.mod ./
# Se houver um go.sum, descomente a linha abaixo
# COPY go.sum ./
RUN go mod download

# Copia o código fonte e compila
COPY . .
RUN go build -o gbdotlive main.go

# Estágio de Execução (Imagem final leve)
FROM debian:bullseye-slim

# Instala as bibliotecas de runtime necessárias para rodar o binário
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
# Copy only necessary files
COPY --from=builder /app/gbdotlive .
COPY ["pkmc (patched).gbc", "."]
COPY gb.svg .
# Create snapshots directory
RUN mkdir snapshots 

# Expose default port
EXPOSE 8000

# Command to start the static server with your ROM on port 8000
# Substitute "pokemon_crystal.gbc" with the real name of your file
CMD ["./gbdotlive", "-S", "-p", "8000", "-r", "pkmc (patched).gbc"]