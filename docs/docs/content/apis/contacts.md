# Contacts API

The Contacts API allows querying, creating, updating, and deleting contacts (subscribers) for listmonk campaigns and cold outreach sequences.

## Data Model

### Contact Object (`models.Subscriber`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | number | Unique contact subscriber ID |
| `uuid` | string | Unique contact UUID string |
| `name` | string | Full name of contact |
| `email` | string | Primary email address |
| `status` | string | Contact status (`enabled`, `disabled`, `blocklisted`) |
| `lists` | object[] | Array of subscribed list objects |
| `attribs` | object | Custom JSON key-value attributes metadata |
| `created_at` | string | ISO 8601 creation timestamp |
| `updated_at` | string | ISO 8601 last update timestamp |

### Contact Sequence Membership Object (`models.SequenceContact`)
| Field | Type | Description |
| :--- | :--- | :--- |
| `sequence_id` | number | ID of enrolled sequence |
| `subscriber_id` | number | Contact subscriber ID |
| `email_id` | number | Assigned sender email account ID |
| `waha_session` | string | Assigned WhatsApp session name |
| `status` | string | Membership state (`scheduled`, `in_progress`, `replied`, `finished`, `opted_out`) |
| `current_step` | number | Current step position in sequence workflow |
| `next_send_at` | string | ISO 8601 timestamp for next scheduled message dispatch |
| `last_read_at` | string | ISO 8601 timestamp when last message was read/opened |
| `last_clicked_at` | string | ISO 8601 timestamp when link was clicked |

---

## Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/contacts` | Query and retrieve contacts |
| `GET` | `/api/contacts/{id}` | Retrieve a specific contact by ID |
| `GET` | `/api/contacts/{id}/sequences` | Retrieve sequence memberships for a contact |
| `POST` | `/api/contacts` | Create a new contact |
| `PUT` | `/api/contacts/{id}` | Update an existing contact |
| `PUT` | `/api/contacts/sequences` | Modify contact sequence memberships (`enroll`, `disenroll`, `pause`) |
| `PUT` | `/api/contacts/query/sequences` | Modify contact sequence memberships by query filter |
| `DELETE` | `/api/contacts/{id}` | Delete a specific contact |
| `DELETE` | `/api/contacts` | Bulk delete contacts |

---

### GET /api/contacts

Retrieve paginated contacts or query contacts with filters.

#### Parameters
| Parameter | Type | Description |
| :--- | :--- | :--- |
| `query` | string | Search query matching name, email, or phone |
| `page` | number | Page number (default: 1) |
| `per_page` | number | Results per page (default: 20) |

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/contacts?query=john'
```

#### Example Response
```json
{
  "data": {
    "results": [
      {
        "id": 1,
        "uuid": "8f88a442-3b2d-4c4f-a991-8843c0892019",
        "name": "John Doe",
        "email": "john@example.com",
        "status": "enabled",
        "attribs": {
          "city": "New York",
          "company": "Acme Inc"
        },
        "created_at": "2026-08-10T12:00:00Z",
        "updated_at": "2026-08-10T12:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "per_page": 20
  }
}
```

---

### GET /api/contacts/{id}

Retrieve details for a specific contact by ID.

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/contacts/1'
```

---

### POST /api/contacts

Create a new contact.

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `email` | string | Yes | Email address of the contact |
| `name` | string | No | Full name of the contact |
| `status` | string | No | Contact status (`enabled`, `disabled`, `blocklisted`) |
| `lists` | number[] | No | List of list IDs to subscribe the contact to |
| `attribs` | object | No | Arbitrary JSON key-value metadata attributes |

#### Example Request
```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/contacts' \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "alice@example.com",
    "name": "Alice Smith",
    "status": "enabled",
    "lists": [1],
    "attribs": {
      "company": "Tech Corp",
      "role": "CTO"
    }
  }'
```

---

### PUT /api/contacts/{id}

Update an existing contact by ID.

#### Example Request
```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/contacts/1' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice Johnson",
    "status": "enabled",
    "attribs": {
      "company": "Tech Corp",
      "role": "VP Engineering"
    }
  }'
```

---

### GET /api/contacts/{id}/sequences

Retrieve sequence memberships and state positions for a contact.

#### Example Request
```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/contacts/1/sequences'
```

---

### PUT /api/contacts/sequences

Modify contact sequence memberships (enroll contacts into target sequences, disenroll, or pause).

#### Parameters
| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `action` | string | Yes | Action to perform: `add` (or `enroll`), `remove` (or `disenroll`), `pause` |
| `contact_ids` | number[] | Yes | Array of contact IDs |
| `target_sequence_ids` | number[] | Yes | Array of sequence IDs |
| `status` | string | No | Sequence contact initial status (default: `scheduled`) |

#### Example Request
```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/contacts/sequences' \
  -H 'Content-Type: application/json' \
  -d '{
    "action": "add",
    "contact_ids": [1, 2, 3],
    "target_sequence_ids": [10, 20],
    "status": "scheduled"
  }'
```

---

### DELETE /api/contacts/{id}

Delete a contact by ID.

#### Example Request
```shell
curl -u "username:token" -X DELETE 'http://localhost:9000/api/contacts/1'
```
