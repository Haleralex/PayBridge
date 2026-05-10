$jwtSecret = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })

flyctl secrets set --app paybridge-api `
  PAYBRIDGE_DATABASE_HOST="paybridge-db.flycast" `
  PAYBRIDGE_DATABASE_USER="postgres" `
  PAYBRIDGE_DATABASE_PASSWORD="pGN8fD9mbVl2di7" `
  PAYBRIDGE_DATABASE_DATABASE="paybridge" `
  PAYBRIDGE_AUTH_JWT_SECRET="$jwtSecret" `
  PAYBRIDGE_AUTH_TELEGRAM_BOT_TOKEN="8327261140:AAG6bsJfnEUMlJAKcsZEZyvbYRJLsewn8dI" `
  PAYBRIDGE_TELEMETRY_OTLP_ENDPOINT="https://otlp-gateway-prod-ap-southeast-1.grafana.net/otlp" `
  "OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic MTYyODc3NTpnbGNfZXlKdklqb2lNVGMyTURFNU9DSXNJbTRpT2lKd1lYbGZZbkpwWkdkbFgzUnZhMlZ1SWl3aWF5STZJa000TWxVeE4zWjBkMVF3T0VsdFJUQXpRVTlUTUZnemNTSXNJbTBpT25zaWNpSTZJbkJ5YjJSdFlYQXRjMjkxZEdobFlYTjBMVEVpZlgwPQ==" `
  "DATABASE_URL=postgres://postgres:pGN8fD9mbVl2di7@paybridge-db.flycast:5432/paybridge?sslmode=require"
