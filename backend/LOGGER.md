---

# 📋 JSON Logging Standards (Grafana Optimized)

This document serves as a guide for developers to generate standardized logs. These standards ensure that logs are automatically processed by **Loki** and accurately visualized within the **Apps Activity Log** dashboard.

---

## 1. Core Fields (Mandatory)

Each log entry must be a **single-line JSON object** containing:

| Field       | Type        | Description                |
| ----------- | ----------- | -------------------------- |
| `timestamp` | ISO8601 UTC | Log timestamp              |
| `level`     | String      | `info`, `warn`, `error`    |
| `message`   | String      | Short event description    |
| `path`      | String      | Endpoint or component name |
| `requestId` | UUID        | Request trace identifier   |
| `method`    | String      | HTTP verb or `INTERNAL`    |

---

## 2. Request Logging (`info`)

Used to record inbound requests.

* **Example Output:**
```json
{
  "timestamp": "2026-01-15T14:00:01.123Z",
  "level": "info",
  "requestId": "a3b2-c4d5-e6f7",
  "method": "POST",
  "path": "/api/v1/orders",
  "message": "Incoming request to create order",
  "attributes": {
    "user_id": 1024,
    "order_type": "smartphone"
  }
}
```

---

## 3. Database Query Logging (`info`)

Must use exact `message`: `"Executed query"` for dashboard compatibility.

* **Example Output:**
```json
{
  "timestamp": "2026-01-15T14:05:10.456Z",
  "level": "info",
  "requestId": "a3b2-c4d5-e6f7",
  "method": "INTERNAL",
  "path": "Repository/OrderStore",
  "message": "Executed query",
  "attributes": {
    "query": "SELECT id FROM boards WHERE id = $1 AND user_id = $2",
    "duration_ms": 3,
    "rows_affected": 1
  }
}
```

---

## 4. Warning Logging (`warn`)

Used for degraded behavior that does not interrupt execution.

* **Example Output:**
```json
{
  "timestamp": "2026-01-15T14:10:00.123Z",
  "level": "warn",
  "requestId": "w1-x2-y3",
  "method": "GET",
  "path": "/api/v1/checkout",
  "message": "Slow query detected",
  "metrics": {
    "executionTimeMs": 1200,
    "thresholdMs": 1000
  }
}
```

---

## 5. Error Logging (`error`)

Used for failures that cause request errors or system malfunction.

* **Example Output:**
```json
{
  "timestamp": "2026-01-15T14:10:05.999Z",
  "level": "error",
  "requestId": "e1-r2-r3",
  "method": "POST",
  "path": "/api/v1/payment",
  "message": "Payment Gateway Connection Refused",
  "error": {
    "code": "PG_503",
    "details": "Downstream service unavailable",
    "stack": "Error: Connect ETIMEDOUT 10.20.30.40:443"
  }
}
```

---

## 6. Runtime Output (K3s Requirement)

Applications must emit logs to **stdout** for ingestion by Promtail or Fluent-Bit.

---