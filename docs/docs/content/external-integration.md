# Integrating with external systems

In many environments, a mailing list manager's subscriber database is not run independently but as a part of an existing customer database or a CRM. There are multiple ways of keeping listmonk in sync with external systems.

## Using APIs

The [subscriber APIs](apis/subscribers.md) offers several APIs to manipulate the subscribers database, like addition, updation, and deletion. For bulk synchronisation, a CSV can be generated (and optionally zipped) and posted to the import API.

## Interacting directly with the DB

listmonk uses tables with simple schemas to represent subscribers (`subscribers`), lists (`lists`), and subscriptions (`subscriber_lists`). It is easy to add, update, and delete subscriber information directly with the database tables for advanced usecases. See the [table schemas](https://github.com/knadh/listmonk/blob/master/schema.sql) for more information.

## Frappe / ERPNext CRM Integration & Deep Research

When CRM integration is enabled in listmonk settings (`crm.enabled = true`), listmonk dispatches an automated deep research request to Frappe CRM immediately upon a subscriber's enrollment into a sequence campaign.

### Deep Research Dispatch Workflow

1. **Immediate Enrollment Dispatch**: When a subscriber is enrolled into a sequence campaign, listmonk sets `campaign_subscribers.status = 'waiting'` and dispatches an HTTP POST request to `{crm.base_url}/api/method/frappe_listmonk.deep_research.get`.
2. **Authentication**: Requests pass Frappe API token authentication headers:
   `Authorization: token {api_key}:{api_secret}`
3. **Payload Structure**:
   listmonk packages unmutated `campaign_subscriber` and `subscriber` data models:

```json
{
  "campaign_subscriber": {
    "campaign_id": 42,
    "subscriber_id": 100,
    "email_id": 5,
    "from_address": "sales@example.com",
    "whatsapp_id": "session_default",
    "user_id": 12,
    "status": "waiting",
    "current_step": 0,
    "next_send_at": "2026-08-25T10:00:00Z",
    "last_read_at": null,
    "last_clicked_at": null,
    "last_message_id": null,
    "last_thread_msg_id": null,
    "created_at": "2026-08-25T10:00:00Z"
  },
  "subscriber": {
    "id": 100,
    "uuid": "ea06b2e7-4b08-4697-bcfc-2a5c6dde8f1c",
    "email": "john@example.com",
    "name": "John Doe",
    "phone": "+14155552671",
    "crm_id": "crm_sub_789",
    "attribs": {
      "company": "Acme Corp",
      "city": "San Francisco"
    },
    "status": "enabled",
    "created_at": "2026-08-25T10:00:00Z",
    "updated_at": "2026-08-25T10:00:00Z"
  },
  "campaign_id": 42,
  "list_ids": [1, 2]
}
```

### Resume Callback Endpoint

When Frappe CRM completes AI deep research and web scraping, it updates subscriber attributes via `PATCH /api/subscribers/{subscriber_id}` and posts a callback to listmonk to resume sequence step 1 dispatch:

- **Endpoint**: `POST /api/campaigns/{campaign_id}/subscribers/{subscriber_id}`
- **Request Body**:
  ```json
  {
    "status": "scheduled"
  }
  ```
- **Response**:
  ```json
  {
    "data": {
      "status": "ok",
      "message": "Campaign subscriber status updated to scheduled, sequence resumed"
    }
  }
  ```
- **Behavior**: Sets `campaign_subscribers.status = 'scheduled'` and `next_send_at = NOW()`, causing the sequence worker to pick up the subscriber on the next execution loop.
