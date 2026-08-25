# API / Users

| Method | Endpoint                                        | Description               |
| :----- | :---------------------------------------------- | :------------------------ |
| GET    | [/api/users](#get-apiusers)                     | Retrieve all users.       |
| GET    | [/api/users/{user_id}](#get-apiusersuser_id)    | Retrieve a specific user. |
| POST   | [/api/users](#post-apiusers)                    | Create a new user.        |
| PUT    | [/api/users/{user_id}](#put-apiusersuser_id)    | Update a user profile.    |
| DELETE | [/api/users/{user_id}](#delete-apiusersuser_id) | Delete a specific user.   |
| DELETE | [/api/users](#delete-apiusers)                  | Delete multiple users.    |

______________________________________________________________________

#### GET /api/users

Retrieve all users.

##### Example Request

```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/users'
```

##### Example Response

```json
{
  "data": [
    {
      "id": 1,
      "username": "admin",
      "name": "Admin User",
      "email": "admin@example.com",
      "phone": "+14155550200",
      "type": "user",
      "user_role_id": 1,
      "list_role_id": null,
      "status": "enabled",
      "email_id": 10,
      "waha_session": "sales_session_a",
      "signature": "<p>Best regards,<br/>Admin User</p>",
      "crm_id": "crm_usr_101",
      "attribs": {
        "bio": "Senior Sales Executive",
        "department": "Enterprise"
      },
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

______________________________________________________________________

#### GET /api/users/{user_id}

Retrieve a specific user by ID.

##### Example Request

```shell
curl -u "username:token" -X GET 'http://localhost:9000/api/users/1'
```

##### Example Response

```json
{
  "data": {
    "id": 1,
    "username": "admin",
    "name": "Admin User",
    "email": "admin@example.com",
    "phone": "+14155550200",
    "type": "user",
    "user_role_id": 1,
    "list_role_id": null,
    "status": "enabled",
    "email_id": 10,
    "waha_session": "sales_session_a",
    "signature": "<p>Best regards,<br/>Admin User</p>"
  }
}
```

______________________________________________________________________

#### POST /api/users

Create a new user.

##### Request Body

```json
{
  "username": "salesrep1",
  "name": "Alice Sales Rep",
  "email": "alice@company.com",
  "phone": "+918935885359",
  "type": "user",
  "user_role_id": 2,
  "status": "enabled",
  "email_id": 10,
  "waha_session": "sales_session_a",
  "signature": "<p>Best regards,<br/>Alice</p>",
  "crm_id": "crm_usr_101",
  "attribs": {
    "bio": "Senior Sales Executive",
    "department": "Enterprise"
  }
}
```

##### Example Request

```shell
curl -u "username:token" -X POST 'http://localhost:9000/api/users' \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "salesrep1",
    "name": "Alice Sales Rep",
    "email": "alice@company.com",
    "phone": "+918935885359",
    "type": "user",
    "user_role_id": 2,
    "status": "enabled"
  }'
```

##### Example Response

```json
{
  "data": {
    "id": 2,
    "username": "salesrep1",
    "name": "Alice Sales Rep",
    "email": "alice@company.com",
    "phone": "+918935885359",
    "type": "user",
    "user_role_id": 2,
    "status": "enabled"
  }
}
```

______________________________________________________________________

#### PUT /api/users/{user_id}

Update an existing user profile.

##### Example Request

```shell
curl -u "username:token" -X PUT 'http://localhost:9000/api/users/2' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice Sales Manager",
    "status": "enabled"
  }'
```

##### Example Response

```json
{
  "data": {
    "id": 2,
    "username": "salesrep1",
    "name": "Alice Sales Manager",
    "email": "alice@company.com",
    "status": "enabled"
  }
}
```

______________________________________________________________________

#### DELETE /api/users/{user_id}

Delete a specific user by ID.

##### Example Request

```shell
curl -u "username:token" -X DELETE 'http://localhost:9000/api/users/2'
```

##### Example Response

```json
{
  "data": true
}
```

______________________________________________________________________

#### DELETE /api/users

Delete multiple users by query parameter.

##### Example Request

```shell
curl -u "username:token" -X DELETE 'http://localhost:9000/api/users?id=2&id=3'
```

##### Example Response

```json
{
  "data": true
}
```
