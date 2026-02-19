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
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/gbdotlive .
# Certifique-se de que a ROM está no repositório para ser copiada
COPY . . 

# Exponha a porta padrão do servidor
EXPOSE 1989

# Comando para iniciar o servidor estático com a sua ROM
# Substitua "pokemon_crystal.gbc" pelo nome real do seu arquivo
CMD ["./gbdotlive", "-S", "-r", "pokemon_crystal.gbc"]