# APIs

All features that are available on the listmonk dashboard are also available as REST-like HTTP APIs that can be interacted with directly. Request and response bodies are JSON. This allows easy scripting of listmonk and integration with other systems, for instance, synchronisation with external subscriber databases.

!!! note
    If you come across API calls that are yet to be documented, please consider contributing to docs.


## Auth

HTTP API requests support BasicAuth, `Authorization: token` headers, `Authorization: Bearer` headers, and `X-API-Key` headers.

### API Credentials Standard
All API authentication requires two credentials:
- **`username`**: The API user account username string (e.g. `admin` or `api_user`).
- **`token`**: The secret access token string generated in Admin -> Users.

### Generating API Credentials
API user accounts and tokens are managed under **Admin -> Users**:
1. Navigate to **Users** in the listmonk dashboard.
2. Create or edit an API user account (`Type = API`) and assign role permissions (e.g. `contacts:manage`, `sequences:send`, `webhooks:manage`).
3. Copy the generated secret access token upon creation.

### Authentication Examples

##### BasicAuth example
```shell
curl -u "username:token" http://localhost:9000/api/lists
```

##### Authorization token header example
```shell
curl -H "Authorization: token username:token" http://localhost:9000/api/lists
```

##### Authorization Bearer header example
```shell
curl -H "Authorization: Bearer token" http://localhost:9000/api/lists
```

##### X-API-Key header example
```shell
curl -H "X-API-Key: token" http://localhost:9000/api/lists
```

---

## Authorization & Role-Based Access Control (RBAC)

listmonk enforces granular permission checks on every API route via authorization middleware (`Auth.Perm`):

- **User Roles**: Group domain-specific permissions (e.g. `sequences:get`, `sequences:send`, `contacts:manage`, `schedules:manage`, `whatsapp:manage`, `emails:manage`).
- **List Roles**: Per-list read (`lists:get`) and write (`lists:manage`) restrictions.
- **Permission Overrides**: `lists:get_all` or `lists:manage_all` permissions in a User Role override per-list restrictions.
- **Superadmin Bypass**: The default Superadmin user role (`user_role_id = 1`) possesses full access across all endpoints.

______________________________________________________________________

## Response structure

### Successful request

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "data": {}
}
```

All responses from the API server are JSON with the content-type application/json unless explicitly stated otherwise. A successful 200 OK response always has a JSON response body with a status key with the value success. The data key contains the full response payload.

### Failed request

```http
HTTP/1.1 500 Server error
Content-Type: application/json

{
    "message": "Error message"
}
```

A failure response is preceded by the corresponding 40x or 50x HTTP header. There may be an optional `data` key with additional payload.

### Timestamps

All timestamp fields are in the format `2019-01-01T09:00:00.000000+05:30`. The seconds component is suffixed by the milliseconds, followed by the `+` and the timezone offset.

### Common HTTP error codes

| Code  |                                                                             |
| ----- | ----------------------------------------------------------------------------|
|  400  | Missing or bad request parameters or values                                 |
|  403  | Session expired or invalidate. Must relogin                                 |
|  404  | Request resource was not found                                              |
|  405  | Request method (GET, POST etc.) is not allowed on the requested endpoint    |
|  410  | The requested resource is gone permanently                                  |
|  422  | Unprocessable entity. Unable to process request as it contains invalid data |
|  429  | Too many requests to the API (rate limiting)                                |
|  500  | Something unexpected went wrong                                             |
|  502  | The backend OMS is down and the API is unable to communicate with it        |
|  503  | Service unavailable; the API is down                                        |
|  504  | Gateway timeout; the API is unreachable                                     |


## OpenAPI (Swagger) spec

The auto-generated OpenAPI (Swagger) specification site for the APIs are available at [**listmonk.app/docs/swagger**](https://listmonk.app/docs/swagger/)

