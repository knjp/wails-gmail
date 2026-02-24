# --- ステージ1: フロントエンドのビルド ---
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# --- ステージ2: Goサーバーのビルド ---
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
# CGOを無効にして、どこでも動く「純粋なGoバイナリ」を作る
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 🌟 serverタグを付けてビルド！
RUN go build -tags server -o gmail-app main_server.go app.go auth.go procdb.go

# --- ステージ3: 実行用（超軽量イメージ） ---
FROM alpine:latest
WORKDIR /root/
COPY --from=go-builder /app/gmail-app .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# 🌟 設定ファイル用のディレクトリを作成
RUN mkdir -p config db

# 8080ポートを公開
EXPOSE 8080

# 実行！
CMD ["./gmail-app"]
