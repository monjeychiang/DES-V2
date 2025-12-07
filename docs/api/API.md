# DES-V2 API Documentation

## Overview

The DES-V2 Trading System exposes a RESTful HTTP API and WebSocket interface for real-time communication.

**Base URL**: `http://localhost:8080` (configurable via `PORT` environment variable)

---

## REST API Endpoints

### Health Check

**GET** `/health`

Check if the API server is running.

**Response**:
```json
{
  "status": "ok"
}
```

**Example**:
```bash
curl http://localhost:8080/health
```

---

## WebSocket API

### Real-Time Event Stream

**WS** `/ws`

Subscribe to real-time trading events including price ticks, signals, orders, and alerts.

**Connection**:
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

**Event Types**:

#### 1. Price Tick Event
```json
{
  "type": "price_tick",
  "data": {
    "symbol": "BTCUSDT",
    "price": 45000.50,
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

#### 2. Strategy Signal Event
```json
{
  "type": "strategy_signal",
  "data": {
    "strategy": "grid_strategy",
    "symbol": "BTCUSDT",
    "action": "BUY",
    "size": 0.001,
    "note": "Grid level triggered"
  }
}
```

#### 3. Order Event
```json
{
  "type": "order",
  "data": {
    "id": "uuid-string",
    "symbol": "BTCUSDT",
    "side": "BUY",
    "price": 45000.00,
    "qty": 0.001,
    "status": "FILLED",
    "timestamp": "2024-01-15T10:30:05Z"
  }
}
```

#### 4. Risk Alert Event
```json
{
  "type": "risk_alert",
  "data": {
    "message": "Position limit exceeded",
    "severity": "high"
  }
}
```

**Usage Example** (JavaScript):
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  console.log('Connected to DES-V2');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.type, data.data);
  
  switch(data.type) {
    case 'price_tick':
      updatePriceDisplay(data.data);
      break;
    case 'strategy_signal':
      showSignalNotification(data.data);
      break;
    case 'order':
      updateOrderBook(data.data);
      break;
    case 'risk_alert':
      showAlert(data.data.message);
      break;
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Connection closed');
};
```

---

## Future Endpoints (Planned)

### Strategy Management

**POST** `/api/strategies`
- Start a new strategy instance

**DELETE** `/api/strategies/:id`
- Stop a running strategy

**GET** `/api/strategies`
- List all strategies

### Backtesting

**POST** `/api/backtest/start`
- Start a backtest job

**GET** `/api/backtest/results/:id`
- Get backtest results

### Order Management

**GET** `/api/orders`
- Get order history

**GET** `/api/positions`
- Get current positions

### System Configuration

**GET** `/api/config`
- Get current system configuration

**PUT** `/api/config`
- Update system configuration

---

## Authentication

> [!IMPORTANT]
> Authentication is planned but not yet implemented. Currently, the API is open for development purposes.

Future authentication will use JWT tokens:
```
Authorization: Bearer <jwt-token>
```

---

## Rate Limiting

Currently no rate limiting is enforced. This will be added in production.

---

## Error Responses

Standard error response format:
```json
{
  "error": "Error description",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

Common HTTP status codes:
- `200 OK` - Success
- `400 Bad Request` - Invalid request
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Permission denied
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Testing the API

### Using cURL

```bash
# Health check
curl http://localhost:8080/health

# WebSocket (using websocat)
websocat ws://localhost:8080/ws
```

### Using PowerShell

```powershell
# Health check
Invoke-RestMethod -Uri "http://localhost:8080/health"

# WebSocket
# Requires WebSocket client library or tool
```

---

## Next Steps

1. Implement additional REST endpoints for strategy and order management
2. Add authentication middleware
3. Implement request validation
4. Add API versioning (`/api/v1/...`)
5. Generate OpenAPI/Swagger documentation
