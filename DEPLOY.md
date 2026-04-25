# Deployment Guide

## Option 1: Koyeb (Recommended)

### Step 1: Push to GitHub
```bash
git add .github/workflows/deploy.yml
git commit -m "chore: add deploy workflow"
git push origin main
```

### Step 2: Connect to Koyeb
1. Go to https://app.koyeb.com
2. Click "Create App"
3. Select "GitHub" source
4. Connect your repository
5. Select the `main` branch
6. Configure:
   - **Build Command:** `docker build -t app .`
   - **Run Command:** `./devops-toolkit`
   - **Port:** `3000`
7. Add Environment Variables:
   - `DEVOPS_SERVER_PORT=3000`
   - `DEVOPS_DATABASE_HOST` (your PostgreSQL host)
   - `DEVOPS_DATABASE_PORT=5432`
   - `DEVOPS_DATABASE_USER=postgres`
   - `DEVOPS_DATABASE_PASSWORD=your_password`
   - `DEVOPS_DATABASE_NAME=devops_toolkit`

### Step 3: Add PostgreSQL Database
Koyeb free tier doesn't include a persistent database. Options:
- Use free PostgreSQL from [Supabase](https://supabase.com) or [Neon](https://neon.tech)
- Set up PostgreSQL on Oracle Cloud Always Free

---

## Option 2: Oracle Cloud Always Free (永久免费)

Oracle Cloud provides always-free ARM compute and Always Free PostgreSQL.

### Sign Up
1. Go to https://www.oracle.com/cloud/free/
2. Create account (requires credit card but is genuinely free)
3. Create a free ARM VM or use their managed PostgreSQL

### Deploy
```bash
# SSH to your Oracle Cloud VM
# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker ubuntu

# Pull and run
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

## Option 3: Google Cloud Run (每月免费额度)

### Build and Push to GHCR
```bash
# Set up GCP project and enable Container Registry
gcloud auth configure-docker ghcr.io

# Build and push
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

For the deploy workflow, add these in GitHub Settings > Secrets:

| Secret | Value |
|--------|-------|
| `KOYEB_API_TOKEN` | Your Koyeb API token |
| `KOYEB_DATABASE_PASSWORD` | PostgreSQL password |

---

## Local Testing with Docker

```bash
# Build
docker build -t devops-toolkit .

# Run with PostgreSQL
docker run -d \
  --name devops-toolkit \
  -p 3000:3000 \
  --link devops-postgres \
  devops-toolkit

# Or use docker-compose
docker-compose up -d
```
