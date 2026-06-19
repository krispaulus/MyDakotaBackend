FROM golang:1.24-bookworm

ENV GOTOOLCHAIN=auto

# 1. Install SQL Server Driver & Tools
RUN apt-get update && apt-get install -y curl gnupg2 \
    && mkdir -p /etc/apt/keyrings \
    && curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor -o /etc/apt/keyrings/microsoft.gpg \
    && echo "deb [arch=amd64,arm64,armhf signed-by=/etc/apt/keyrings/microsoft.gpg] https://packages.microsoft.com/debian/12/prod bookworm main" > /etc/apt/sources.list.d/mssql-release.list \
    && apt-get update \
    && ACCEPT_EULA=Y apt-get install -y msodbcsql17 unixodbc-dev

# 2. JURUS SUPER PAMUNGKAS: Aktifkan Legacy Provider & TLS 1.0
# Kita rewrite openssl.cnf agar mengizinkan algoritma jadul
RUN echo 'openssl_conf = openssl_init\n\
    \n\
    [openssl_init]\n\
    providers = provider_sect\n\
    ssl_conf = ssl_sect\n\
    \n\
    [provider_sect]\n\
    default = default_sect\n\
    legacy = legacy_sect\n\
    \n\
    [default_sect]\n\
    activate = 1\n\
    \n\
    [legacy_sect]\n\
    activate = 1\n\
    \n\
    [ssl_sect]\n\
    system_default = system_default_sect\n\
    \n\
    [system_default_sect]\n\
    MinProtocol = TLSv1\n\
    CipherString = DEFAULT@SECLEVEL=0' > /etc/ssl/openssl.cnf

WORKDIR /app

# 3. Handle Go Modules
COPY go.mod go.sum ./
RUN go mod download

# 4. Copy Source Code
COPY . .

# 5. Buat folder uploads (PENTING)
# Perintah ini memastikan folder ada di dalam container
RUN mkdir -p /app/uploads

# 6. Build Binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 7. Environment Variables
ENV GODEBUG=tlsrsakex=1,tls10default=1

EXPOSE 8080

CMD ["./main"]