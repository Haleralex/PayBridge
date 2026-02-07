# PayBridge Telegram Mini App

## Quick Start

### 1. Serve the webapp locally

```bash
# Using Python
cd webapp
python -m http.server 3000

# Or using Node.js
npx serve -p 3000

# Or using Go
go run webapp/server.go
```

### 2. Create Telegram Bot

1. Open [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow instructions
3. Send `/mybots` → Select your bot → Bot Settings → Menu Button
4. Set Menu Button URL: `https://your-domain.com` (use ngrok for testing)

### 3. For local testing with ngrok

```bash
# Terminal 1: Run PayBridge API
docker-compose up -d

# Terminal 2: Serve webapp
cd webapp && python -m http.server 3000

# Terminal 3: Expose with ngrok
ngrok http 3000
```

Copy the ngrok HTTPS URL and set it as Menu Button URL in BotFather.

## Features

- View wallet balance
- Deposit funds (Credit)
- Withdraw funds (Debit)
- Transfer between wallets
- Transaction history
- Telegram theme support
- Haptic feedback

## Production Setup

1. **Deploy webapp** to Vercel/Netlify/Cloudflare Pages
2. **Update API_BASE** in index.html to your production API URL
3. **Configure CORS** to allow your webapp domain
4. **Implement Telegram auth** - validate `initData` on backend

### Telegram Auth (Production)

Replace mock token with real Telegram auth:

```javascript
// In index.html
const tg = window.Telegram.WebApp;
const initData = tg.initData; // Send this to backend for validation

// Backend validates using bot token
const AUTH_TOKEN = await getTokenFromBackend(initData);
```

## File Structure

```
webapp/
├── index.html      # Main Mini App (single file, all-in-one)
├── README.md       # This file
└── server.go       # Optional Go static server
```
