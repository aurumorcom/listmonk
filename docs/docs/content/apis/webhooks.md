# Webhooks API

The Webhooks API allows registering outbound webhook endpoints, managing event subscriptions (subscribers, contacts, sequences, campaigns), querying delivery logs, and testing webhook triggers.

## Data Model

### Webhook Object (`models.Webhook`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | number | Unique auto-incrementing endpoint identifier |
| `name` | string | Descriptive name for the webhook target |
| `url` | string | Destination HTTP(S) endpoint URL receiving JSON payloads |
| `secret` | string | Secret key for computing `Listmonk-Signature` HMAC SHA256 signature header |
| `events` | string[] | Array of subscribed event triggers (e.g., `contact.created`, `sequence.step_executed`) |
| `enabled` | boolean | Active status flag (`true` or `false`) |
| `created_at` | string | ISO 8601 timestamp of creation |
| `updated_at` | string | ISO 8601 timestamp of last update |

### Webhook Event Payload Object (`models.Event`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | string | Unique event snapshot UUID (e.g. `evt_12345678`) |
| `event` | string | Name of triggered event (e.g. `subscriber.created`, `contact.updated`) |
| `created_at` | string | ISO 8601 UTC timestamp of event dispatch |
| `data` | object | Complete snapshot object of the resource (Subscriber, Contact, Campaign, Sequence) |

---

## Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/webhooks` | Retrieve all registered webhook endpoints |
| `POST` | `/api/webhooks` | Create a new webhook endpoint |
| `PUT` | `/api/webhooks/{id}` | Update an existing webhook endpoint |
| `DELETE` | `/api/webhooks/{id}` | Delete a webhook endpoint |
| `GET` | `/api/webhooks/logs` | Query paginated webhook delivery logs |
| `POST` | `/api/webhooks/test` | Dispatch a test event payload to a target URL |

---

### GET /api/webhooks

Retrieve registered webhook endpoints.

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/webhooks'
```

#### Example Response
```json
{
  "data": [
    {
      "id": 1,
      "name": "n8n Production Receiver",
      "url": "https://n8n.example.com/webhook/listmonk",
      "secret": "whsec_1234567890",
      "events": [
        "subscriber.created",
        "subscriber.updated",
        "contact.created",
        "contact.updated",
        "sequence.step_executed"
      ],
      "enabled": true,
      "created_at": "2026-08-10T12:00:00Z",
      "updated_at": "2026-08-10T12:00:00Z"
    }
  ]
}
```

---

### POST /api/webhooks

Register a new outbound webhook target.

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Name identifier for the webhook endpoint |
| `url` | string | Yes | HTTP(S) endpoint URL receiving JSON payloads |
| `secret` | string | No | Secret key used for `Listmonk-Signature` HMAC SHA256 header |
| `events` | string[] | Yes | Array of subscribed event triggers (e.g. `subscriber.created`, `contact.created`, `sequence.step_executed`) |
| `enabled` | boolean | No | Enable/disable status (default: `true`) |

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/webhooks' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Zapier Integration",
    "url": "https://hooks.zapier.com/hooks/catch/123456/abcde",
    "secret": "whsec_zapier_secret",
    "events": [
      "subscriber.created",
      "subscriber.unsubscribed",
      "contact.created"
    ],
    "enabled": true
  }'
```

---

### PUT /api/webhooks/{id}

Update an existing outbound webhook target configuration.

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Updated name identifier for the webhook endpoint |
| `url` | string | Yes | Updated HTTP(S) endpoint URL receiving JSON payloads |
| `secret` | string | No | Updated secret key used for `Listmonk-Signature` HMAC SHA256 header |
| `events` | string[] | Yes | Array of subscribed event triggers |
| `enabled` | boolean | No | Enable/disable status |

#### Example Request
```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/webhooks/1' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Zapier Integration Updated",
    "url": "https://hooks.zapier.com/hooks/catch/123456/abcde",
    "secret": "whsec_new_secret",
    "events": [
      "subscriber.created",
      "contact.created"
    ],
    "enabled": true
  }'
```

---

### DELETE /api/webhooks/{id}

Delete an existing outbound webhook endpoint by ID.

#### Example Request
```shell
curl -u "username:token" -X DELETE 'http://localhost:9000/api/webhooks/1'
```

---

### POST /api/webhooks/test

Send an immediate test event payload to verify endpoint URL connectivity and signature computation.

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `url` | string | Yes | Target endpoint URL |
| `secret` | string | No | Optional secret key for HMAC verification |
| `event_type` | string | Yes | Trigger event name (e.g. `subscriber.created`, `contact.created`) |

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/webhooks/test' \
  -H 'Content-Type: application/json' \
  -d '{
    "url": "https://n8n.example.com/webhook/listmonk",
    "secret": "whsec_test",
    "event_type": "subscriber.created"
  }'
```

---

### GET /api/webhooks/logs

Retrieve paginated history of dispatched webhook logs and HTTP response status codes.

#### Parameters
| Parameter | Type | Description |
| :--- | :--- | :--- |
| `page` | number | Page number (default: 1) |
| `per_page` | number | Logs per page (default: 20) |

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/webhooks/logs?page=1&per_page=10'
```
