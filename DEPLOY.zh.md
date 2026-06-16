# 部署指南

> 📖 **语言:** [English](DEPLOY.md) · [简体中文](DEPLOY.zh.md) — 项目的文档与 UI 双语处理详见 [i18n 规范](docs/i18n/SPEC.md)。

## Option 1：Koyeb（推荐）

### Step 1：推送到 GitHub
```bash
git add .github/workflows/deploy.yml
git commit -m "chore: add deploy workflow"
git push origin main
```

### Step 2：连接到 Koyeb
1. 访问 https://app.koyeb.com
2. 点击 "Create App"
3. 选择 "GitHub" source
4. 关联你的仓库
5. 选择 `main` 分支
6. 配置：
   - **Build Command：** `docker build -t app .`
   - **Run Command：** `./devops-toolkit`
   - **Port：** `3000`
7. 添加环境变量：
   - `DEVOPS_SERVER_PORT=3000`
   - `DEVOPS_DATABASE_HOST`（你的 PostgreSQL host）
   - `DEVOPS_DATABASE_PORT=5432`
   - `DEVOPS_DATABASE_USER=postgres`
   - `DEVOPS_DATABASE_PASSWORD=your_password`
   - `DEVOPS_DATABASE_NAME=devops_toolkit`

### Step 3：添加 PostgreSQL 数据库
Koyeb 免费套餐不包含持久化数据库。使用 Supabase 免费套餐：

1. 在 https://app.supabase.com 创建账号
2. 新建项目（免费套餐：500MB 数据库，2GB 存储）
3. 进入 **Settings → Database** 查看连接信息：
   - Host：`db.[your-project-ref].supabase.co`
   - Port：`5432`
   - User：`postgres`
   - Password：你的 Supabase DB 密码
   - Database：`postgres`

4. 在 Koyeb 环境变量中添加：
   - `DEVOPS_DATABASE_HOST=db.[your-project-ref].supabase.co`
   - `DEVOPS_DATABASE_PORT=5432`
   - `DEVOPS_DATABASE_USER=postgres`
   - `DEVOPS_DATABASE_PASSWORD=your_supabase_password`
   - `DEVOPS_DATABASE_NAME=postgres`

---

## Option 2：Oracle Cloud Always Free（永久免费）

Oracle Cloud 提供永久免费的 ARM 计算与 Always Free PostgreSQL。

### Sign Up
1. 访问 https://www.oracle.com/cloud/free/
2. 创建账号（需要信用卡，但确实免费）
3. 创建免费 ARM VM，或使用它的托管 PostgreSQL

### Deploy
```bash
# SSH 到你的 Oracle Cloud VM
# 安装 Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker ubuntu

# 拉取并运行
docker run -d \
  --name devops-toolkit \
  -p 3000:3000 \
  -e DEVOPS_SERVER_PORT=3000 \
  -e DEVOPS_DATABASE_HOST=your-db-host \
  -e DEVOPS_DATABASE_PORT=5432 \
  -e DEVOPS_DATABASE_USER=postgres \
  -e DEVOPS_DATABASE_PASSWORD=xxx \
  -e DEVOPS_DATABASE_NAME=devops_toolkit \
  ghcr.io/lkad/devops-toolkit:latest
```

---

## Option 3：Google Cloud Run（每月免费额度）

### Build and Push to GHCR
```bash
# 设置 GCP project 并启用 Container Registry
gcloud auth configure-docker ghcr.io

# 构建并推送
docker build -t ghcr.io/lkad/devops-toolkit:latest .
docker push ghcr.io/lkad/devops-toolkit:latest
```

### Deploy to Cloud Run
```bash
gcloud run deploy devops-toolkit \
  --image=ghcr.io/lkad/devops-toolkit:latest \
  --platform=managed \
  --region=us-central1 \
  --allow-unauthenticated \
  --port=3000 \
  --set-env-vars=DEVOPS_DATABASE_HOST=your-db
```

---

## GitHub Actions Secrets Required

针对 deploy workflow，在 GitHub Settings > Secrets 中添加以下内容：

| Secret | Value |
|--------|-------|
| `KOYEB_API_TOKEN` | 你的 Koyeb API token |
| `KOYEB_DATABASE_PASSWORD` | PostgreSQL 密码 |

---

## Local Testing with Docker

```bash
# 构建
docker build -t devops-toolkit .

# 使用 PostgreSQL 运行
docker run -d \
  --name devops-toolkit \
  -p 3000:3000 \
  --link devops-postgres \
  devops-toolkit

# 或者使用 docker-compose
docker-compose up -d
```
