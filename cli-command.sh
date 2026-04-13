# Deploy application
idpctl deploy \
  --name my-app \
  --image nginx:1.25 \
  --replicas 3 \
  --cpu-request 100m \
  --cpu-limit 500m \
  --memory-request 128Mi \
  --memory-limit 512Mi \
  --env KEY=VALUE \
  --dry-run

# Application management
idpctl app list
idpctl app get my-app
idpctl app delete my-app
idpctl app scale my-app --replicas 5

# Logs
idpctl logs my-app
idpctl logs my-app --follow
idpctl logs my-app --tail 200 --since 1h

# Cost
idpctl cost estimate --name my-app
idpctl cost history --namespace default --days 30
idpctl cost anomalies --namespace default

# Environment variables
idpctl env set my-app KEY=VALUE
idpctl env list my-app
idpctl env delete my-app KEY

# Status
idpctl status
idpctl status --namespace production