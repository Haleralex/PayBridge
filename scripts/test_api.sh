#!/bin/bash
# PayBridge API Test Script
# Usage: ./scripts/test_api.sh [BASE_URL]
#
# Default: http://localhost:8080

set -e

BASE_URL="${1:-http://localhost:8080}"
AUTH_HEADER="Authorization: Bearer test-token"

echo "========================================"
echo "PayBridge API Test Suite"
echo "Base URL: $BASE_URL"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() { echo -e "${GREEN}[OK]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# ============================================
# Health Checks
# ============================================
echo "--- Health Checks ---"

echo -n "Health endpoint: "
HEALTH=$(curl -s "$BASE_URL/health")
if echo "$HEALTH" | grep -q "healthy"; then
    success "Service is healthy"
else
    fail "Health check failed: $HEALTH"
fi

echo -n "Liveness probe: "
LIVE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/live")
if [ "$LIVE" = "200" ]; then
    success "Liveness OK"
else
    fail "Liveness failed: HTTP $LIVE"
fi

echo -n "Readiness probe: "
READY=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/ready")
if [ "$READY" = "200" ]; then
    success "Readiness OK"
else
    fail "Readiness failed: HTTP $READY"
fi

echo ""

# ============================================
# User Operations
# ============================================
echo "--- User Operations ---"

# Create User
echo -n "Creating user: "
USER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/users" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test'$(date +%s)'@example.com",
        "full_name": "John Doe"
    }')

USER_ID=$(echo "$USER_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$USER_ID" ]; then
    success "User created: $USER_ID"
else
    fail "Failed to create user: $USER_RESPONSE"
    USER_ID=""
fi

# Get User
if [ -n "$USER_ID" ]; then
    echo -n "Getting user: "
    GET_USER=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/users/$USER_ID")
    if echo "$GET_USER" | grep -q "$USER_ID"; then
        success "User retrieved"
    else
        fail "Failed to get user: $GET_USER"
    fi
fi

# List Users
echo -n "Listing users: "
LIST_USERS=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/users?limit=10")
if echo "$LIST_USERS" | grep -q "users"; then
    success "Users listed"
else
    fail "Failed to list users: $LIST_USERS"
fi

echo ""

# ============================================
# Wallet Operations
# ============================================
echo "--- Wallet Operations ---"

# Create Wallet
if [ -n "$USER_ID" ]; then
    echo -n "Creating wallet: "
    WALLET_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "user_id": "'"$USER_ID"'",
            "currency_code": "USD",
            "wallet_type": "personal"
        }')

    WALLET_ID=$(echo "$WALLET_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$WALLET_ID" ]; then
        success "Wallet created: $WALLET_ID"
    else
        fail "Failed to create wallet: $WALLET_RESPONSE"
        WALLET_ID=""
    fi
fi

# Get Wallet
if [ -n "$WALLET_ID" ]; then
    echo -n "Getting wallet: "
    GET_WALLET=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/wallets/$WALLET_ID")
    if echo "$GET_WALLET" | grep -q "$WALLET_ID"; then
        success "Wallet retrieved"
    else
        fail "Failed to get wallet: $GET_WALLET"
    fi
fi

# List Wallets
echo -n "Listing wallets: "
LIST_WALLETS=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/wallets?limit=10")
if echo "$LIST_WALLETS" | grep -q "wallets"; then
    success "Wallets listed"
else
    fail "Failed to list wallets: $LIST_WALLETS"
fi

echo ""

# ============================================
# Financial Operations
# ============================================
echo "--- Financial Operations ---"

# Credit Wallet
if [ -n "$WALLET_ID" ]; then
    echo -n "Crediting wallet (+500.00): "
    IDEMPOTENCY_KEY=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "test-key-$(date +%s)")
    CREDIT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets/$WALLET_ID/credit" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "amount": "500.00",
            "idempotency_key": "'"$IDEMPOTENCY_KEY"'",
            "description": "Initial deposit"
        }')

    if echo "$CREDIT_RESPONSE" | grep -q "available_balance"; then
        success "Wallet credited"
        echo "       Balance: $(echo "$CREDIT_RESPONSE" | grep -o '"available_balance":"[^"]*"' | cut -d'"' -f4)"
    else
        fail "Failed to credit wallet: $CREDIT_RESPONSE"
    fi

    # Debit Wallet
    echo -n "Debiting wallet (-100.00): "
    IDEMPOTENCY_KEY2=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "test-key2-$(date +%s)")
    DEBIT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets/$WALLET_ID/debit" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "amount": "100.00",
            "idempotency_key": "'"$IDEMPOTENCY_KEY2"'",
            "description": "Test withdrawal"
        }')

    if echo "$DEBIT_RESPONSE" | grep -q "available_balance"; then
        success "Wallet debited"
        echo "       Balance: $(echo "$DEBIT_RESPONSE" | grep -o '"available_balance":"[^"]*"' | cut -d'"' -f4)"
    else
        fail "Failed to debit wallet: $DEBIT_RESPONSE"
    fi
fi

echo ""

# ============================================
# Transfer Between Wallets
# ============================================
echo "--- Transfer Operations ---"

# Create second wallet for transfer
if [ -n "$USER_ID" ]; then
    echo -n "Creating second wallet (EUR): "
    WALLET2_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "user_id": "'"$USER_ID"'",
            "currency_code": "USD",
            "wallet_type": "personal"
        }')

    WALLET2_ID=$(echo "$WALLET2_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$WALLET2_ID" ]; then
        success "Second wallet created: $WALLET2_ID"
    else
        fail "Failed to create second wallet: $WALLET2_RESPONSE"
    fi
fi

# Transfer between wallets
if [ -n "$WALLET_ID" ] && [ -n "$WALLET2_ID" ]; then
    echo -n "Transferring 50.00 USD: "
    IDEMPOTENCY_KEY3=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "test-key3-$(date +%s)")
    TRANSFER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets/$WALLET_ID/transfer" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "destination_wallet_id": "'"$WALLET2_ID"'",
            "amount": "50.00",
            "idempotency_key": "'"$IDEMPOTENCY_KEY3"'",
            "description": "Test transfer"
        }')

    if echo "$TRANSFER_RESPONSE" | grep -q "transaction_id"; then
        success "Transfer completed"
        info "Source balance: $(echo "$TRANSFER_RESPONSE" | grep -o '"source_wallet":{[^}]*"available_balance":"[^"]*"' | grep -o '"available_balance":"[^"]*"' | cut -d'"' -f4)"
        info "Destination balance: $(echo "$TRANSFER_RESPONSE" | grep -o '"destination_wallet":{[^}]*"available_balance":"[^"]*"' | grep -o '"available_balance":"[^"]*"' | cut -d'"' -f4)"
    else
        fail "Failed to transfer: $TRANSFER_RESPONSE"
    fi
fi

echo ""

# ============================================
# Transaction History
# ============================================
echo "--- Transaction History ---"

echo -n "Listing transactions: "
LIST_TX=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/transactions?limit=10")
if echo "$LIST_TX" | grep -q "transactions"; then
    success "Transactions listed"
    COUNT=$(echo "$LIST_TX" | grep -o '"total":' | head -1)
    info "Total transactions in response"
else
    fail "Failed to list transactions: $LIST_TX"
fi

if [ -n "$WALLET_ID" ]; then
    echo -n "Wallet transactions: "
    WALLET_TX=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/wallets/$WALLET_ID/transactions")
    if echo "$WALLET_TX" | grep -q "transactions"; then
        success "Wallet transactions retrieved"
    else
        fail "Failed to get wallet transactions: $WALLET_TX"
    fi
fi

echo ""

# ============================================
# Error Cases
# ============================================
echo "--- Error Handling ---"

echo -n "Invalid wallet ID: "
ERROR_RESPONSE=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/api/v1/wallets/invalid-uuid")
if echo "$ERROR_RESPONSE" | grep -q "error"; then
    success "Error handled correctly"
else
    fail "Unexpected response: $ERROR_RESPONSE"
fi

echo -n "Insufficient balance: "
if [ -n "$WALLET2_ID" ]; then
    IDEMPOTENCY_KEY4=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "test-key4-$(date +%s)")
    INSUF_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/wallets/$WALLET2_ID/debit" \
        -H "$AUTH_HEADER" \
        -H "Content-Type: application/json" \
        -d '{
            "amount": "999999.00",
            "idempotency_key": "'"$IDEMPOTENCY_KEY4"'",
            "description": "Should fail"
        }')

    if echo "$INSUF_RESPONSE" | grep -qi "insufficient\|error"; then
        success "Insufficient balance handled"
    else
        fail "Unexpected response: $INSUF_RESPONSE"
    fi
fi

echo ""
echo "========================================"
echo "Test suite completed!"
echo "========================================"
