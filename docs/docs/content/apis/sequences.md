# Sequences API

The Sequences API enables management of automated multi-step cold outreach sequences, steps, and sender reassignments.

## Data Model

### Sequence Object (`models.Sequence`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | number | Unique sequence identifier |
| `uuid` | string | Sequence UUID string |
| `name` | string | Name of the outreach sequence |
| `description` | string | Description and campaign objectives |
| `status` | string | Sequence status (`active`, `paused`, `archived`, `cancelled`) |
| `schedule_id` | number | Associated sending schedule ID |
| `send_window` | object | Daily sending window time range configuration |
| `email_ids` | number[] | Array of sender email account IDs |
| `waha_sessions` | string[] | Array of WhatsApp session names |
| `archive` | boolean | Web archive status |
| `archive_template_id` | number | Template ID used for web archive view |
| `archive_slug` | string | Custom URL slug for public web archive |
| `archive_meta` | object | Web archive metadata JSON object |

### Sequence Step Object (`models.SequenceStep`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | number | Sequence step ID |
| `sequence_id` | number | Associated parent sequence ID |
| `step_number` | number | Step sequence order number (1, 2, 3...) |
| `delay_seconds` | number | Time delay in seconds before executing step after previous trigger |
| `messenger` | string | Target delivery messenger channel (`email`, `waha`) |
| `condition` | string | Execution trigger condition (`always`, `if_read`, `if_not_read`, `if_clicked`) |
| `subject` | string | Email or message subject line |
| `body` | string | Content body template |
| `email_type` | string | Email thread type (`New Thread`, `Reply`) |
| `template_id` | number | Optional template ID |
| `media_ids` | number[] | Array of attached media file IDs |

---

## Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/sequences` | Retrieve all sequences |
| `GET` | `/api/sequences/analytics` | Retrieve sequence conversion analytics |
| `GET` | `/api/sequences/{id}` | Retrieve a specific sequence |
| `GET` | `/api/sequences/{id}/preview` | Preview sequence step template content |
| `POST` | `/api/sequences` | Create a new sequence |
| `POST` | `/api/sequences/{id}/test` | Dispatch a test sequence step payload |
| `PUT` | `/api/sequences/{id}` | Update an existing sequence |
| `PUT` | `/api/sequences/{id}/status` | Update sequence status (`active`, `paused`, `archived`, `cancelled`) |
| `PUT` | `/api/sequences/{id}/archive` | Update sequence archive settings |
| `DELETE` | `/api/sequences/{id}` | Delete a sequence |
| `DELETE` | `/api/sequences` | Bulk delete sequences by IDs |
| `GET` | `/api/sequences/{id}/steps` | Retrieve step configuration for a sequence |
| `POST` | `/api/sequences/{id}/steps` | Save step configuration for a sequence |
| `PUT` | `/api/sequences/{id}/contacts/{sub_id}/reassign` | Reassign locked sender channel for a contact |

---

### GET /api/sequences

Retrieve all configured sequences.

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/sequences'
```

#### Example Response
```json
{
  "data": [
    {
      "id": 1,
      "uuid": "7a32b110-1234-4a56-8a90-123456789abc",
      "name": "Outreach Campaign Q3",
      "description": "Cold email sequence targeting B2B leads",
      "status": "active",
      "schedule_id": 1,
      "archive": false,
      "created_at": "2026-08-10T12:00:00Z",
      "updated_at": "2026-08-10T12:00:00Z"
    }
  ]
}
```

---

### POST /api/sequences

Create a new sequence.

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Sequence name |
| `description` | string | No | Sequence description |
| `status` | string | No | Sequence status (`active`, `paused`) |
| `schedule_id` | number | No | Sending schedule ID |
| `archive` | boolean | No | Enable public archive |
| `archive_template_id` | number | No | Archive template ID |
| `archive_slug` | string | No | Custom archive URL slug |
| `archive_meta` | object | No | Archive metadata JSON object |

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/sequences' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "SaaS Founder Outreach",
    "description": "Personalized 3-step sequence",
    "status": "active",
    "schedule_id": 1
  }'
```

---

### POST /api/sequences/{id}/steps

Save multi-step workflow configuration for a sequence.

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/sequences/1/steps' \
  -H 'Content-Type: application/json' \
  -d '{
    "steps": [
      {
        "step_number": 1,
        "delay_days": 0,
        "messenger": "email",
        "condition": "always",
        "subject": "Quick question about {{ .Subscriber.Attribs.company }}",
        "body": "<p>Hi {{ .Subscriber.FirstName }}, ...</p>"
      },
      {
        "step_number": 2,
        "delay_days": 2,
        "messenger": "email",
        "condition": "if_not_read",
        "subject": "Following up on my last email",
        "body": "<p>Hi {{ .Subscriber.FirstName }}, following up on {{ .Step1.subject }}...</p>"
      }
    ]
  }'
```

---

### GET /api/sequences/analytics

Retrieve aggregated conversion metrics and funnel performance across sequences.

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/sequences/analytics'
```

---

### PUT /api/sequences/{id}/status

Update sequence status (`active`, `paused`, `archived`, `cancelled`).

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `status` | string | Yes | New status value (`active`, `paused`, `archived`, `cancelled`) |

#### Example Request
```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/sequences/1/status' \
  -H 'Content-Type: application/json' \
  -d '{"status": "paused"}'
```

---

### PUT /api/sequences/{id}/archive

Update sequence web archive configuration.

#### Example Request
```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/sequences/1/archive' \
  -H 'Content-Type: application/json' \
  -d '{
    "archive": true,
    "archive_slug": "outreach-q3"
  }'
```

---

### POST /api/sequences/{id}/test

Dispatch immediate test payload for sequence steps.

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/sequences/1/test' \
  -H 'Content-Type: application/json' \
  -d '{"email": "test@example.com"}'
```

---

### DELETE /api/sequences

Bulk delete sequences by IDs.

#### Example Request
```shell
curl -u "username:token" -X DELETE 'http://localhost:9000/api/sequences?id=1&id=2'
```
